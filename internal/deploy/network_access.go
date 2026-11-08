package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

var applicationNetworkID = regexp.MustCompile(`^[a-f0-9]{24}$`)

func ValidateNetworkAccess(application string, access *store.NetworkAccess) error {
	if len(access.Applications) > 64 || len(access.Ports) > 64 {
		return errors.New("Networking is limited to 64 applications and 64 ports")
	}
	seen := map[string]bool{}
	for _, id := range access.Applications {
		if !applicationNetworkID.MatchString(id) || id == application || seen[id] {
			return errors.New("Choose each other application once")
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for i := range access.Ports {
		p := &access.Ports[i]
		p.Protocol = strings.ToLower(strings.TrimSpace(p.Protocol))
		if p.Protocol != "tcp" && p.Protocol != "udp" {
			return errors.New("Choose TCP or UDP")
		}
		if p.HostPort < 1024 || p.HostPort > 65535 || p.ContainerPort < 1 || p.ContainerPort > 65535 {
			return errors.New("Public ports must be between 1024 and 65535 and container ports between 1 and 65535")
		}
		if p.BindIP == "" {
			p.BindIP = "0.0.0.0"
		}
		if _, err := netip.ParseAddr(p.BindIP); err != nil {
			return errors.New("Enter a valid bind IP address")
		}
		if p.Service != "" && !serviceName.MatchString(p.Service) {
			return errors.New("Choose a valid Compose service")
		}
		key := strconv.Itoa(p.HostPort) + "/" + p.Protocol
		if seen[key] {
			return errors.New("A public port can only be mapped once")
		}
		seen[key] = true
	}
	return nil
}

// ValidatePublishedPorts checks stopped and unmanaged Docker containers and the
// host socket table before any application or infrastructure mutation.
func (r *DockerRuntime) ValidatePublishedPorts(ctx context.Context, application string, ports []store.PublishedPort) error {
	access := store.NetworkAccess{Ports: append([]store.PublishedPort{}, ports...)}
	if err := ValidateNetworkAccess(application, &access); err != nil {
		return err
	}
	containers, err := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	for _, p := range access.Ports {
		for _, reserved := range r.Config.ReservedPorts {
			if p.HostPort == reserved {
				return errors.New("This port is reserved for Shipper")
			}
		}
		if p.HostPort == r.Config.HTTPPort || p.HostPort == r.Config.HTTPSPort || p.HostPort == 8080 {
			return errors.New("This port is reserved for Shipper")
		}
		ownBinding := false
		for _, c := range containers.Items {
			for _, bound := range c.Ports {
				if int(bound.PublicPort) != p.HostPort || string(bound.Type) != p.Protocol {
					continue
				}
				if owned(c.Labels, r.Instance, application) {
					ownBinding = ownBinding || c.State == "running"
					continue
				}
				return fmt.Errorf("Port %d/%s is already in use", p.HostPort, p.Protocol)
			}
			// Docker can omit published ports on stopped containers.
			if c.State != "running" {
				info, e := r.Client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
				if e != nil {
					return e
				}
				if info.Container.HostConfig == nil {
					continue
				}
				for port, bindings := range info.Container.HostConfig.PortBindings {
					if strings.HasSuffix(port.String(), "/"+p.Protocol) {
						for _, b := range bindings {
							if b.HostPort == strconv.Itoa(p.HostPort) {
								if owned(c.Labels, r.Instance, application) {
									ownBinding = true
								} else {
									return fmt.Errorf("Port %d/%s is reserved by another container", p.HostPort, p.Protocol)
								}
							}
						}
					}
				}
			}
		}
		if ownBinding {
			continue
		}
		address := net.JoinHostPort(p.BindIP, strconv.Itoa(p.HostPort))
		if p.Protocol == "tcp" {
			listener, e := net.Listen("tcp", address)
			if e != nil {
				return fmt.Errorf("Port %d/tcp is unavailable", p.HostPort)
			}
			listener.Close()
		} else {
			listener, e := net.ListenPacket("udp", address)
			if e != nil {
				return fmt.Errorf("Port %d/udp is unavailable", p.HostPort)
			}
			listener.Close()
		}
	}
	return nil
}

func (r *DockerRuntime) applicationNetwork(app string) string { return r.prefix() + "-app-" + app }
func (r *DockerRuntime) ensureApplicationNetwork(ctx context.Context, app string) error {
	name := r.applicationNetwork(app)
	info, err := r.Client.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	if err == nil {
		if !owned(info.Network.Labels, r.Instance, app) {
			return errors.New("Application network ownership check failed")
		}
	} else if errdefs.IsNotFound(err) {
		labels := r.labels(app, "")
		labels["io.shipper.resource"] = "network"
		if _, err = r.Client.NetworkCreate(ctx, name, client.NetworkCreateOptions{Driver: "bridge", Labels: labels}); err != nil {
			return err
		}
	} else {
		return err
	}
	// The proxy is the only infrastructure container allowed onto private app networks.
	proxy, err := r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{})
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !owned(proxy.Container.Config.Labels, r.Instance, "") || proxy.Container.Config.Labels["io.shipper.infrastructure"] != "proxy" {
		return errors.New("Proxy ownership check failed")
	}
	if proxy.Container.NetworkSettings != nil {
		if _, ok := proxy.Container.NetworkSettings.Networks[name]; ok {
			return nil
		}
	}
	_, err = r.Client.NetworkConnect(ctx, name, client.NetworkConnectOptions{Container: proxy.Container.ID})
	return err
}
func (r *DockerRuntime) applicationEndpoints(ctx context.Context, d store.Deployment) (map[string]*network.EndpointSettings, error) {
	endpoints := map[string]*network.EndpointSettings{}
	for _, app := range append([]string{d.ApplicationID}, d.Config.Networking.Applications...) {
		if err := r.ensureApplicationNetwork(ctx, app); err != nil {
			return nil, err
		}
		endpoint := &network.EndpointSettings{}
		if app == d.ApplicationID {
			endpoint.Aliases = []string{"site-" + d.ID}
		}
		endpoints[r.applicationNetwork(app)] = endpoint
	}
	return endpoints, nil
}
func publishedBindings(ports []store.PublishedPort) network.PortMap {
	bindings := network.PortMap{}
	for _, p := range ports {
		ip := p.BindIP
		if ip == "" {
			ip = "0.0.0.0"
		}
		key := network.MustParsePort(strconv.Itoa(p.ContainerPort) + "/" + p.Protocol)
		bindings[key] = append(bindings[key], network.PortBinding{HostIP: netip.MustParseAddr(ip), HostPort: strconv.Itoa(p.HostPort)})
	}
	return bindings
}

// PrepareNetworkCutover releases fixed host ports before replacement. The
// deployment journal retains PreviousID so rollback can restart it after failure.
func (r *DockerRuntime) PrepareNetworkCutover(ctx context.Context, d store.Deployment) error {
	if err := r.ValidatePublishedPorts(ctx, d.ApplicationID, d.Config.Networking.Ports); err != nil {
		return err
	}
	if d.PreviousID == "" || d.TaskCommand != nil {
		return nil
	}
	items, err := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+d.ApplicationID).Add("label", "io.shipper.deployment.id="+d.PreviousID)})
	if err != nil {
		return err
	}
	for _, c := range items.Items {
		if c.State != "running" || !owned(c.Labels, r.Instance, d.ApplicationID) {
			continue
		}
		if len(c.Ports) == 0 && d.Config.Database == nil {
			continue
		}
		if _, err = r.Client.ContainerStop(ctx, c.ID, client.ContainerStopOptions{Timeout: ptr(StopSeconds(d.Config.Runtime))}); err != nil {
			return err
		}
	}
	return nil
}
func (r *DockerRuntime) RestoreNetworkCutover(ctx context.Context, d store.Deployment) error {
	if d.PreviousID == "" {
		return nil
	}
	items, err := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+d.ApplicationID).Add("label", "io.shipper.deployment.id="+d.PreviousID)})
	if err != nil {
		return err
	}
	for _, c := range items.Items {
		if c.State != "running" && owned(c.Labels, r.Instance, d.ApplicationID) {
			if _, err = r.Client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reconcile stable private names only after durable traffic promotion. Candidate
// containers keep release-specific names while readiness checks run.
func (r *DockerRuntime) reconcileApplicationAliases(ctx context.Context, active map[string]store.Deployment) error {
	for app, d := range active {
		if ReleaseMethod(d) == "compose" {
			if err := r.reconcileComposeNetworks(ctx, d); err != nil {
				return err
			}
			continue
		}
		if d.ContainerID == "" {
			continue
		}
		if err := r.ensureApplicationNetwork(ctx, app); err != nil {
			return err
		}
		name := r.applicationNetwork(app)
		info, err := r.Client.ContainerInspect(ctx, d.ContainerID, client.ContainerInspectOptions{})
		if err != nil {
			return err
		}
		if info.Container.NetworkSettings == nil {
			return errors.New("Application network settings are unavailable")
		}
		if !owned(info.Container.Config.Labels, r.Instance, app) {
			return errors.New("Application network ownership check failed")
		}
		for _, target := range d.Config.Networking.Applications {
			if err := r.ensureApplicationNetwork(ctx, target); err != nil {
				return err
			}
			targetName := r.applicationNetwork(target)
			if info.Container.NetworkSettings.Networks[targetName] == nil {
				if _, err := r.Client.NetworkConnect(ctx, targetName, client.NetworkConnectOptions{Container: d.ContainerID}); err != nil {
					return err
				}
			}
		}
		if info.Container.NetworkSettings.Networks[r.network()] != nil {
			if _, err := r.Client.NetworkDisconnect(ctx, r.network(), client.NetworkDisconnectOptions{Container: d.ContainerID}); err != nil {
				return err
			}
		}
		endpoint := info.Container.NetworkSettings.Networks[name]
		if endpoint != nil {
			present := false
			for _, alias := range endpoint.Aliases {
				present = present || alias == "app-"+app
			}
			if present {
				continue
			}
			if _, err = r.Client.NetworkDisconnect(ctx, name, client.NetworkDisconnectOptions{Container: d.ContainerID}); err != nil {
				return err
			}
		}
		if _, err = r.Client.NetworkConnect(ctx, name, client.NetworkConnectOptions{Container: d.ContainerID, EndpointConfig: &network.EndpointSettings{Aliases: []string{"site-" + d.ID, "app-" + app, "db-" + app}}}); err != nil {
			return err
		}
		// A previous serving release may still drain, but must not answer the stable name.
		items, err := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+app)})
		if err != nil {
			return err
		}
		for _, c := range items.Items {
			if c.ID == d.ContainerID || c.Labels["io.shipper.infrastructure"] != "" {
				continue
			}
			old, e := r.Client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
			if e != nil {
				return e
			}
			if old.Container.NetworkSettings == nil {
				continue
			}
			ep := old.Container.NetworkSettings.Networks[name]
			if ep == nil {
				continue
			}
			stable := false
			for _, alias := range ep.Aliases {
				stable = stable || alias == "app-"+app
			}
			if !stable {
				continue
			}
			if _, e = r.Client.NetworkDisconnect(ctx, name, client.NetworkDisconnectOptions{Container: c.ID}); e != nil {
				return e
			}
			if _, e = r.Client.NetworkConnect(ctx, name, client.NetworkConnectOptions{Container: c.ID, EndpointConfig: &network.EndpointSettings{Aliases: []string{"site-" + c.Labels["io.shipper.deployment.id"]}}}); e != nil {
				return e
			}
		}
	}
	return nil
}

func (r *DockerRuntime) reconcileComposeNetworks(ctx context.Context, d store.Deployment) error {
	if err := r.ensureApplicationNetwork(ctx, d.ApplicationID); err != nil {
		return err
	}
	items, err := r.composeContainers(ctx, d.ApplicationID)
	if err != nil {
		return err
	}
	for _, c := range items {
		if c.Labels["io.shipper.infrastructure"] != "" {
			continue
		}
		info, err := r.Client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
		if err != nil {
			return err
		}
		if info.Container.NetworkSettings == nil {
			continue
		}
		for _, app := range append([]string{d.ApplicationID}, d.Config.Networking.Applications...) {
			if err := r.ensureApplicationNetwork(ctx, app); err != nil {
				return err
			}
			name := r.applicationNetwork(app)
			if info.Container.NetworkSettings.Networks[name] != nil {
				continue
			}
			endpoint := &network.EndpointSettings{}
			if app == d.ApplicationID {
				endpoint.Aliases = composeServiceAliases(d, c.Labels["com.docker.compose.service"])
			}
			if _, err = r.Client.NetworkConnect(ctx, name, client.NetworkConnectOptions{Container: c.ID, EndpointConfig: endpoint}); err != nil {
				return err
			}
		}
		if info.Container.NetworkSettings.Networks[r.network()] != nil {
			if _, err = r.Client.NetworkDisconnect(ctx, r.network(), client.NetworkDisconnectOptions{Container: c.ID}); err != nil {
				return err
			}
		}
	}
	return nil
}

func composeServiceAliases(d store.Deployment, name string) []string {
	aliases := []string{composeAlias(d.ApplicationID, name)}
	primary := ""
	for _, svc := range d.Services {
		if primary == "" {
			primary = svc.Name
		}
		if svc.Port > 0 {
			primary = svc.Name
			break
		}
	}
	if name == primary {
		aliases = append(aliases, "app-"+d.ApplicationID)
	}
	return aliases
}

func (r *DockerRuntime) reuseComposeNetworking(ctx context.Context, d store.Deployment, plan *composePlan) error {
	services, ok := plan.Project["services"].(map[string]any)
	if !ok {
		return errors.New("Compose services are unavailable")
	}
	for _, port := range d.Config.Networking.Ports {
		if _, ok := services[port.Service]; !ok {
			return errors.New("Choose a Compose service for each public port")
		}
	}
	networks, _ := plan.Project["networks"].(map[string]any)
	if networks == nil {
		networks = map[string]any{}
	}
	for key := range networks {
		if strings.HasPrefix(key, "shipper_link_") {
			delete(networks, key)
		}
	}
	for _, app := range append([]string{d.ApplicationID}, d.Config.Networking.Applications...) {
		if err := r.ensureApplicationNetwork(ctx, app); err != nil {
			return err
		}
		key := "shipper_link_" + app
		if app == d.ApplicationID {
			key = "shipper_edge"
		}
		networks[key] = map[string]any{"name": r.applicationNetwork(app), "external": true}
	}
	plan.Project["networks"] = networks
	for name, raw := range services {
		svc, ok := raw.(map[string]any)
		if !ok {
			return errors.New("Compose service is unavailable")
		}
		attachments, _ := svc["networks"].(map[string]any)
		if attachments == nil {
			attachments = map[string]any{}
		}
		for key := range attachments {
			if strings.HasPrefix(key, "shipper_link_") {
				delete(attachments, key)
			}
		}
		attachments["shipper_edge"] = map[string]any{"aliases": composeServiceAliases(d, name)}
		for _, app := range d.Config.Networking.Applications {
			attachments["shipper_link_"+app] = map[string]any{}
		}
		svc["networks"] = attachments
		delete(svc, "ports")
		ports := []any{}
		for _, p := range d.Config.Networking.Ports {
			if p.Service == name {
				ports = append(ports, map[string]any{"target": p.ContainerPort, "published": strconv.Itoa(p.HostPort), "protocol": p.Protocol, "host_ip": p.BindIP})
			}
		}
		if len(ports) > 0 {
			svc["ports"] = ports
		}
	}
	return nil
}
