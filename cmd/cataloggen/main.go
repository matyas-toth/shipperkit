// Command cataloggen builds Shipper's service library manifest from the
// open-source Coolify service templates plus a small curated set.
//
// Usage:
//
//	go run ./cmd/cataloggen -source coolify-service-templates.json -out internal/catalog/catalog.json
//
// The source file is Coolify's generated templates/service-templates.json.
// Coolify templates are Apache-2.0 licensed; see catalog/ATTRIBUTION.md.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v4"
)

type sourceTemplate struct {
	Documentation string   `json:"documentation"`
	Slogan        string   `json:"slogan"`
	Compose       string   `json:"compose"`
	Tags          []string `json:"tags"`
	Category      string   `json:"category"`
	Logo          string   `json:"logo"`
	Port          string   `json:"port"`
	UpdatedAt     string   `json:"template_last_updated_at"`
}

type manifest struct {
	Version   string    `json:"version"`
	Source    string    `json:"source"`
	Generated string    `json:"generated"`
	Services  []service `json:"services"`
}

type service struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Category      string        `json:"category"`
	Tags          []string      `json:"tags,omitempty"`
	Logomark      string        `json:"logomark"`
	Logo          string        `json:"logo,omitempty"`
	Website       string        `json:"website,omitempty"`
	Documentation string        `json:"documentation,omitempty"`
	Source        string        `json:"source"`
	Updated       string        `json:"updated,omitempty"`
	Featured      bool          `json:"featured,omitempty"`
	MainService   string        `json:"mainService"`
	Port          int           `json:"port"`
	HealthPath    string        `json:"healthPath"`
	Support       string        `json:"support"`
	SupportNote   string        `json:"supportNote,omitempty"`
	Compose       string        `json:"compose"`
	Services      []serviceInfo `json:"services"`
	Fields        []field       `json:"fields"`
	Domains       []domain      `json:"domains,omitempty"`
}

type serviceInfo struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Role  string `json:"role"`
	Port  int    `json:"port,omitempty"`
}

type domain struct {
	Key     string `json:"key,omitempty"`
	Service string `json:"service"`
	Port    int    `json:"port"`
	Primary bool   `json:"primary,omitempty"`
}

type field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"`
	Group       string   `json:"group"`
	Default     string   `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Options     []string `json:"options,omitempty"`
	Service     string   `json:"service,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	Generate    string   `json:"generate,omitempty"`
	Advanced    bool     `json:"advanced,omitempty"`
}

var (
	magicReference = regexp.MustCompile(`\$?\b(SERVICE_(?:FQDN|URL|PASSWORD|PASSWORD_64|PASSWORD_32|USER|EMAIL|BASE64|REALBASE64|HEX)_[A-Z0-9_]+)`)
	varReference   = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::([-?])([^}]*))?\}`)
	fqdnVariable   = regexp.MustCompile(`^SERVICE_(FQDN|URL)_(.+?)(?:_([0-9]{2,5}))?$`)
	secretVariable = regexp.MustCompile(`(?:^|_)(SECRET|SECRET_KEY|ENCRYPTION_KEY|JWT|APP_KEY|SALT|HASH_SALT|TOKEN_SECRET|API_TOKEN|PRIVATE_KEY)(?:$|_)`)
)

var featuredOrder = map[string]int{
	"n8n": 0, "grafana": 1, "immich": 2, "supabase": 3, "nocodb": 4, "uptime-kuma": 5,
	"vaultwarden": 6, "ghost": 7, "open-webui": 8, "appwrite": 9, "plane": 10, "docmost": 11,
	"gitea": 12, "forgejo": 13, "homepage": 14, "portainer": 15, "code-server": 16,
	"jellyfin": 17, "nextcloud": 18, "paperless": 19, "actualbudget": 20, "activepieces": 21,
	"wordpress": 22, "postgresql": 23, "mysql": 24, "redis": 25, "outline": 26, "calcom": 27,
}

func featured(id string) bool {
	_, ok := featuredOrder[id]
	return ok
}

func main() {
	source := flag.String("source", "", "Coolify service-templates.json")
	out := flag.String("out", "internal/catalog/catalog.json", "Output manifest")
	flag.Parse()
	if *source == "" {
		fmt.Fprintln(os.Stderr, "cataloggen: -source is required")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*source)
	if err != nil {
		fail(err)
	}
	var templates map[string]sourceTemplate
	if err = json.Unmarshal(raw, &templates); err != nil {
		fail(err)
	}
	ids := make([]string, 0, len(templates))
	for id := range templates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	items := make([]service, 0, len(templates)+16)
	for _, id := range ids {
		item, err := convert(id, templates[id])
		if err != nil {
			fmt.Fprintf(os.Stderr, "cataloggen: skip %s: %v\n", id, err)
			continue
		}
		items = append(items, item)
	}
	// Curated services are added last, then the whole manifest is sorted by
	// name so the library never exposes the upstream ordering.
	items = append(items, curatedServices()...)
	sort.Slice(items, func(i, j int) bool {
		ri, iok := featuredOrder[items[i].ID]
		rj, jok := featuredOrder[items[j].ID]
		if iok != jok {
			return iok
		}
		if iok && jok && ri != rj {
			return ri < rj
		}
		if (items[i].Source == "shipper") != (items[j].Source == "shipper") {
			return items[i].Source == "shipper"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	m := manifest{
		Version:   "2026.11.23",
		Source:    "Coolify service templates and Shipper curated services",
		Generated: "2026-11-23",
		Services:  items,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if err = os.WriteFile(*out, data, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("cataloggen: wrote %d services to %s (%d bytes)\n", len(items), *out, len(data))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "cataloggen:", err)
	os.Exit(1)
}

func convert(id string, t sourceTemplate) (service, error) {
	decoded, err := base64.StdEncoding.DecodeString(t.Compose)
	if err != nil {
		return service{}, err
	}
	var doc map[string]any
	if err = yaml.Unmarshal(decoded, &doc); err != nil {
		return service{}, err
	}
	rawServices, _ := doc["services"].(map[string]any)
	if len(rawServices) == 0 {
		return service{}, errors.New("no services")
	}
	fixTemplate(id, doc)
	transformed, fields, domains, mainService, port, support, note, err := transform(doc)
	if err != nil {
		return service{}, err
	}
	for i := range domains {
		if domains[i].Service == mainService && domains[i].Port > 0 && domains[i].Port != 80 {
			port = domains[i].Port
			break
		}
	}
	if servicePort(rawServices[mainService]) > 0 {
		port = servicePort(rawServices[mainService])
	}
	if port == 0 {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(t.Port)); parseErr == nil && parsed > 0 {
			port = parsed
		}
	}
	for i := range domains {
		if domains[i].Port == 0 {
			domains[i].Port = port
		}
	}
	if len(domains) == 0 && port > 0 && isHTTPPort(port) {
		domains = append(domains, domain{Service: mainService, Port: port, Primary: true})
	}
	if len(domains) > 0 {
		domains[0].Primary = true
	}
	data, err := yaml.Marshal(transformed)
	if err != nil {
		return service{}, err
	}
	infos := serviceInfos(transformed, mainService)
	for i := range infos {
		if infos[i].Name == mainService && infos[i].Port == 0 {
			infos[i].Port = port
		}
	}
	name := displayName(id)
	tags := cleanTags(t.Tags)
	category := displayCategory(t.Category)
	if category == "" {
		category = "Other"
	}
	return service{
		ID:            id,
		Name:          name,
		Description:   strings.TrimSpace(t.Slogan),
		Category:      category,
		Tags:          tags,
		Logomark:      logomark(name),
		Logo:          serviceLogoPath(id, t.Logo),
		Documentation: cleanURL(t.Documentation),
		Source:        "coolify",
		Updated:       t.UpdatedAt,
		Featured:      featured(id),
		MainService:   mainService,
		Port:          port,
		HealthPath:    "/",
		Support:       support,
		SupportNote:   note,
		Compose:       string(data),
		Services:      infos,
		Fields:        fields,
		Domains:       domains,
	}, nil
}

// fixTemplate adjusts templates whose upstream files cannot work as read-only
// Compose configs. Filebrowser writes its database to a file the template
// binds empty, so the database moves to a volume.
func fixTemplate(id string, doc map[string]any) {
	if id != "filebrowser" {
		return
	}
	services, _ := doc["services"].(map[string]any)
	svc, _ := services["filebrowser"].(map[string]any)
	if svc == nil {
		return
	}
	volumes, _ := svc["volumes"].([]any)
	for i, raw := range volumes {
		if v, ok := raw.(map[string]any); ok && v["target"] == "/database.db" {
			volumes[i] = map[string]any{"type": "volume", "source": "filebrowser-database", "target": "/database"}
			if env, ok := svc["environment"].([]any); ok {
				svc["environment"] = append(env, "FB_DATABASE=/database/filebrowser.db")
			}
		}
	}
}

type variableRef struct {
	fallback string
	required bool
}

func transform(doc map[string]any) (map[string]any, []field, []domain, string, int, string, string, error) {
	rawServices, ok := doc["services"].(map[string]any)
	if !ok || len(rawServices) == 0 {
		return nil, nil, nil, "", 0, "", "", errors.New("no services")
	}
	// Clean the top-level document to the three sections Shipper manages.
	out := map[string]any{}
	advanced := []string{}
	for _, key := range []string{"services", "volumes", "networks"} {
		if value, ok := doc[key]; ok {
			out[key] = value
		}
	}
	if volumes, ok := out["volumes"].(map[string]any); ok {
		for key, raw := range volumes {
			volume, _ := raw.(map[string]any)
			if volume == nil {
				volume = map[string]any{}
			}
			delete(volume, "external")
			delete(volume, "name")
			delete(volume, "driver_opts")
			delete(volume, "labels")
			volumes[key] = volume
		}
	}
	if networks, ok := out["networks"].(map[string]any); ok {
		for key, raw := range networks {
			network, _ := raw.(map[string]any)
			if network == nil {
				network = map[string]any{}
			}
			delete(network, "external")
			delete(network, "name")
			delete(network, "driver_opts")
			delete(network, "labels")
			if driver, _ := network["driver"].(string); driver != "" && driver != "bridge" {
				network["driver"] = "bridge"
			}
			networks[key] = network
		}
	}
	if len(rawServices) > 32 {
		return nil, nil, nil, "", 0, "advanced", "This template needs more than 32 services", errors.New("too many services")
	}
	// First pass: remove host-only and unsupported service options. A pasted
	// Compose file cannot read repository files, so a file the template writes
	// inline becomes a Compose config and every other bind becomes a volume.
	files := []fileConfig{}
	for name, raw := range rawServices {
		svc, _ := raw.(map[string]any)
		if svc == nil {
			delete(rawServices, name)
			continue
		}
		if _, ok := svc["build"]; ok {
			delete(svc, "build")
		}
		for _, key := range []string{"container_name", "labels", "exclude_from_hc", "pull_policy"} {
			delete(svc, key)
		}
		if mode, _ := svc["network_mode"].(string); mode != "" {
			if mode == "host" {
				advanced = append(advanced, "host networking")
			}
			delete(svc, "network_mode")
		}
		if flagValue, _ := svc["privileged"].(bool); flagValue {
			advanced = append(advanced, "privileged access")
			delete(svc, "privileged")
		}
		for _, key := range []string{"cap_add", "cap_drop", "devices", "security_opt", "sysctls", "pid", "ipc", "uts", "userns_mode", "volumes_from", "external_links"} {
			if _, ok := svc[key]; ok {
				advanced = append(advanced, key)
				delete(svc, key)
			}
		}
		if deploy, ok := svc["deploy"].(map[string]any); ok {
			delete(deploy, "replicas")
			if len(deploy) == 0 {
				delete(svc, "deploy")
			}
		}
		if _, ok := svc["env_file"]; ok {
			delete(svc, "env_file")
			advanced = append(advanced, "environment file")
		}
		if configs, ok := svc["configs"].([]any); ok {
			svc["configs"] = configs
		}
		if secrets, ok := svc["secrets"].([]any); ok {
			svc["secrets"] = secrets
		}
		if volumes, ok := svc["volumes"].([]any); ok {
			clean := make([]any, 0, len(volumes))
			for index, raw := range volumes {
				volume := cleanVolume(name, raw, index, &advanced, &files)
				if volume != nil {
					clean = append(clean, volume)
				}
			}
			svc["volumes"] = clean
		}
		normalizeEnvironment(name, svc)
		rawServices[name] = svc
	}
	if len(rawServices) == 0 {
		return nil, nil, nil, "", 0, "", "", errors.New("no usable services")
	}
	// Every named volume a service refers to must be declared. Upstream
	// templates sometimes rely on Coolify generating these declarations.
	volumes, _ := out["volumes"].(map[string]any)
	if volumes == nil {
		volumes = map[string]any{}
	}
	for _, raw := range rawServices {
		svc, _ := raw.(map[string]any)
		list, _ := svc["volumes"].([]any)
		for _, item := range list {
			volume, _ := item.(map[string]any)
			source, _ := volume["source"].(string)
			kind, _ := volume["type"].(string)
			if kind == "volume" && source != "" && !strings.Contains(source, "$") {
				if _, ok := volumes[source]; !ok {
					volumes[source] = map[string]any{}
				}
			}
		}
	}
	out["volumes"] = volumes

	// Second pass: resolve every referenced variable into a field.
	refs := map[string]variableRef{}
	walk(out, func(text string) {
		for _, index := range varReference.FindAllStringSubmatchIndex(text, -1) {
			// Compose escapes runtime shell references as $${NAME}. They must
			// reach the container untouched and are not application inputs.
			if index[0] > 0 && text[index[0]-1] == '$' {
				continue
			}
			name := text[index[2]:index[3]]
			operator := ""
			fallback := ""
			if index[4] >= 0 {
				operator = text[index[4]:index[5]]
				fallback = text[index[6]:index[7]]
			}
			previous := refs[name]
			if !previous.required || operator == "?" {
				previous.required = previous.required || operator == "?"
			}
			if previous.fallback == "" && operator != "?" {
				previous.fallback = fallback
			}
			refs[name] = previous
		}
	})
	fields, domains := buildFields(rawServices, refs, accessKeyUsers(rawServices))
	mainService, port := pickMain(rawServices, fields)
	escapeRuntimeStrings(out, refs)
	// File contents reach the container verbatim, so every $ is escaped and no
	// reference inside them becomes a field.
	if len(files) > 0 {
		configs := map[string]any{}
		for _, f := range files {
			svc, _ := rawServices[f.Service].(map[string]any)
			if svc == nil {
				continue
			}
			entry := map[string]any{"source": f.Name, "target": f.Target}
			if strings.HasPrefix(f.Content, "#!") {
				entry["mode"] = 0o755
			}
			list, _ := svc["configs"].([]any)
			svc["configs"] = append(list, entry)
			configs[f.Name] = map[string]any{"content": strings.ReplaceAll(f.Content, "$", "$$")}
		}
		out["configs"] = configs
	}
	for i := range fields {
		if fields[i].Kind == "domain" {
			if fields[i].Service == "" {
				fields[i].Service = mainService
			}
			if fields[i].Service != "" {
				fields[i].Label = "Domain for " + displayName(fields[i].Service)
			}
		}
	}
	for i := range domains {
		if domains[i].Service == "" {
			domains[i].Service = mainService
		}
	}
	support := "ready"
	note := ""
	if len(advanced) > 0 {
		support = "advanced"
		note = "Needs " + humanJoin(unique(advanced)) + " after deploy"
	} else if len(fields) > 24 {
		support = "advanced"
		note = fmt.Sprintf("Review %d configuration fields before deploying", len(fields))
	}
	// Keep the primary domain first so the UI and review surface lead with it.
	sort.SliceStable(domains, func(i, j int) bool {
		if domains[i].Primary != domains[j].Primary {
			return domains[i].Primary
		}
		return domains[i].Service < domains[j].Service
	})
	// Ensure the main service has a domain when it exposes HTTP.
	if port > 0 && isHTTPPort(port) {
		found := false
		for i := range domains {
			if domains[i].Service == mainService {
				domains[i].Primary = true
				found = true
				break
			}
		}
		if !found {
			domains = append([]domain{{Service: mainService, Port: port, Primary: true}}, domains...)
		}
	}
	// Drop fields that reference services that do not exist.
	validServices := map[string]bool{}
	for name := range rawServices {
		validServices[name] = true
	}
	kept := fields[:0]
	for _, f := range fields {
		if f.Kind == "domain" && f.Service != "" && !validServices[f.Service] {
			continue
		}
		if f.Kind == "domain" && f.Service == "" {
			continue
		}
		kept = append(kept, f)
	}
	fields = kept
	out["services"] = rawServices
	return out, fields, domains, mainService, port, support, note, nil
}

func escapeRuntimeStrings(value any, refs map[string]variableRef) {
	switch typed := value.(type) {
	case string:
		return
	case []any:
		for index := range typed {
			if text, ok := typed[index].(string); ok {
				typed[index] = escapeRuntimeString(text, refs)
			} else {
				escapeRuntimeStrings(typed[index], refs)
			}
		}
	case map[string]any:
		for key, item := range typed {
			if text, ok := item.(string); ok {
				typed[key] = escapeRuntimeString(text, refs)
			} else {
				escapeRuntimeStrings(item, refs)
			}
		}
	}
}

// escapeRuntimeString protects container-side shell and template syntax from
// Compose interpolation. Known application variables keep working, while
// runtime expressions such as ${process.env.PORT} become literal text.
func escapeRuntimeString(value string, refs map[string]variableRef) string {
	if !strings.Contains(value, "$") {
		return value
	}
	var out strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '$' {
			out.WriteByte(value[index])
			continue
		}
		if index+1 < len(value) && value[index+1] == '$' {
			out.WriteString("$$")
			index++
			continue
		}
		if index+1 < len(value) && value[index+1] == '{' {
			end := strings.IndexByte(value[index+2:], '}')
			if end < 0 {
				out.WriteByte('$')
				continue
			}
			expression := value[index+2 : index+2+end]
			name := expression
			if cut := strings.IndexAny(name, ":-?"); cut >= 0 {
				name = name[:cut]
			}
			if variableNamePattern.MatchString(name) {
				if _, known := refs[name]; known || strings.HasPrefix(name, "SERVICE_") {
					out.WriteString(value[index : index+3+end])
					index += 2 + end
					continue
				}
			}
			out.WriteString("$$")
			continue
		}
		if index+1 < len(value) {
			next := value[index+1]
			if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || next == '_' {
				end := index + 1
				for end < len(value) {
					ch := value[end]
					if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
						end++
						continue
					}
					break
				}
				name := value[index+1 : end]
				if _, known := refs[name]; known || strings.HasPrefix(name, "SERVICE_") {
					out.WriteString(value[index:end])
					index = end - 1
					continue
				}
				out.WriteString("$$")
				continue
			}
		}
		out.WriteByte('$')
	}
	return out.String()
}

var variableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func normalizeEnvironment(serviceName string, svc map[string]any) {
	raw, ok := svc["environment"]
	if !ok {
		return
	}
	env := map[string]any{}
	switch value := raw.(type) {
	case []any:
		for _, item := range value {
			switch row := item.(type) {
			case string:
				key, val, found := strings.Cut(row, "=")
				if !found {
					val = magicValue(key)
				}
				env[strings.TrimSpace(key)] = environmentValue(val)
			case map[string]any:
				for k, v := range row {
					env[k] = environmentValue(fmt.Sprint(v))
				}
			}
		}
	case map[string]any:
		for key, value := range value {
			if value == nil {
				env[key] = magicValue(key)
				continue
			}
			env[key] = environmentValue(fmt.Sprint(value))
		}
	}
	svc["environment"] = env
}

// environmentValue expands Coolify's magic variables. A value that is only
// $NAME is a setting Coolify lets the user fill in. Docker never expands it,
// so it becomes an optional field instead of reaching the app as literal text.
func environmentValue(value string) string {
	value = expandMagic(value)
	if m := bareVariable.FindStringSubmatch(value); m != nil {
		return "${" + m[1] + ":-}"
	}
	return value
}

var bareVariable = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)$`)

func magicValue(key string) string {
	switch {
	case strings.HasPrefix(key, "SERVICE_FQDN_"),
		strings.HasPrefix(key, "SERVICE_URL_"),
		strings.HasPrefix(key, "SERVICE_PASSWORD_"):
		return "${" + key + "}"
	case strings.HasPrefix(key, "SERVICE_USER_"), strings.HasPrefix(key, "SERVICE_EMAIL_"):
		return "${" + key + "}"
	}
	return ""
}

func expandMagic(value string) string {
	indices := magicReference.FindAllStringIndex(value, -1)
	if len(indices) == 0 {
		return value
	}
	var out strings.Builder
	last := 0
	for _, index := range indices {
		start, end := index[0], index[1]
		out.WriteString(value[last:start])
		match := value[start:end]
		switch {
		case strings.HasPrefix(match, "${"):
			out.WriteString(match)
		case strings.HasPrefix(match, "$"):
			out.WriteString("${" + strings.TrimPrefix(match, "$") + "}")
		case start >= 2 && value[start-2:start] == "${":
			out.WriteString(match)
		case start >= 1 && value[start-1] == '$':
			out.WriteString("${" + match + "}")
		default:
			out.WriteString("${" + match + "}")
		}
		last = end
	}
	out.WriteString(value[last:])
	return out.String()
}

// fileConfig is a file a template writes inline. It becomes a Compose config,
// because a named volume mounted on a file path is an empty directory.
type fileConfig struct {
	Service, Name, Target, Content string
}

func cleanVolume(service string, raw any, index int, advanced *[]string, files *[]fileConfig) any {
	switch value := raw.(type) {
	case string:
		parts := strings.Split(value, ":")
		if len(parts) == 1 {
			return namedVolume(service, index, parts[0], "")
		}
		source, target := parts[0], parts[1]
		mode := ""
		if len(parts) > 2 {
			mode = parts[2]
		}
		return convertSource(source, target, mode, service, index, advanced, false)
	case map[string]any:
		target, _ := value["target"].(string)
		source, _ := value["source"].(string)
		kind, _ := value["type"].(string)
		directory := value["is_directory"] == true || value["isDirectory"] == true
		delete(value, "is_directory")
		delete(value, "isDirectory")
		delete(value, "create_host_path")
		if content, ok := value["content"]; ok && !directory && target != "" {
			text := ""
			if content != nil {
				text = fmt.Sprint(content)
			}
			*files = append(*files, fileConfig{Service: service, Name: strings.ReplaceAll(service, "_", "-") + "-file-" + strconv.Itoa(index), Target: target, Content: text})
			return nil
		}
		delete(value, "content")
		if kind == "bind" {
			mode := ""
			if readOnly, _ := value["read_only"].(bool); readOnly {
				mode = "ro"
			}
			return convertSource(source, target, mode, service, index, advanced, directory)
		}
		if kind == "tmpfs" {
			return value
		}
		if source == "" {
			return namedVolume(service, index, target, "")
		}
		if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") || strings.Contains(source, "$") {
			mode := ""
			if readOnly, _ := value["read_only"].(bool); readOnly {
				mode = "ro"
			}
			return convertSource(source, target, mode, service, index, advanced, directory)
		}
		return value
	}
	return nil
}

func convertSource(source, target, mode, service string, index int, advanced *[]string, directory bool) any {
	if source == "" {
		return namedVolume(service, index, target, mode)
	}
	if strings.Contains(source, "$") {
		return namedVolume(service, index, target, mode)
	}
	if isHostAccess(source) {
		*advanced = append(*advanced, "host access")
		return nil
	}
	if source == "/etc/localtime" || source == "/dev/urandom" {
		return nil
	}
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") || strings.HasPrefix(source, "/") {
		// A file bound without contents cannot be created here, and a volume
		// in its place would be an empty directory. The image's own copy is used.
		if !directory && fileSource.MatchString(source) {
			return nil
		}
		return namedVolume(service, index, target, mode)
	}
	if strings.HasPrefix(source, "${") || strings.Contains(source, "$") {
		return map[string]any{"type": "volume", "source": source, "target": target, "read_only": mode == "ro"}
	}
	return namedVolume(service, index, target, mode)
}

func namedVolume(service string, index int, target, mode string) map[string]any {
	volume := map[string]any{
		"type":   "volume",
		"source": "data-" + strings.ReplaceAll(service, "_", "-") + "-" + strconv.Itoa(index),
		"target": target,
	}
	if mode == "ro" {
		volume["read_only"] = true
	}
	return volume
}

func isHostAccess(source string) bool {
	for _, needle := range []string{"docker.sock", "podman.sock", "/run/dbus", "/var/lib/docker", "/var/run/docker", "/proc", "/sys", "/dev"} {
		if strings.Contains(source, needle) {
			return true
		}
	}
	return false
}

// fileSource matches a bind source that names a single file.
var fileSource = regexp.MustCompile(`(?i)[^/]\.(toml|ya?ml|json|conf|cfg|ini|env|js|ts|php|xml|sh|txt|lua|py|properties|rb|pem|key|crt|htpasswd|sqlite3?|db|sql|css|html)$`)

// accessKeyUsers lists the SERVICE_USER_ variables a template uses as an
// access key. Those are credentials, not names: Garage rejects a key shorter
// than 8 characters, and a readable default like "minio" is guessable.
func accessKeyUsers(services map[string]any) map[string]bool {
	users := map[string]bool{}
	for _, raw := range services {
		svc, _ := raw.(map[string]any)
		env, _ := svc["environment"].(map[string]any)
		for key, value := range env {
			if !accessKeyName.MatchString(key) {
				continue
			}
			for _, match := range varReference.FindAllStringSubmatch(fmt.Sprint(value), -1) {
				if strings.HasPrefix(match[1], "SERVICE_USER_") {
					users[match[1]] = true
				}
			}
		}
	}
	return users
}

var accessKeyName = regexp.MustCompile(`(?i)ACCESS_?KEY|KEY_ID|MINIO_ROOT_USER`)

func buildFields(services map[string]any, refs map[string]variableRef, accessKeys map[string]bool) ([]field, []domain) {
	fields := []field{}
	domains := []domain{}
	seen := map[string]bool{}
	add := func(f field) {
		if seen[f.Key] {
			return
		}
		seen[f.Key] = true
		fields = append(fields, f)
	}
	names := make([]string, 0, len(refs))
	for name := range refs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		reference := refs[name]
		fallback := reference.fallback
		switch {
		case strings.HasPrefix(name, "SERVICE_FQDN_"):
			service, port := resolveService(name, "SERVICE_FQDN_", services)
			domains = append(domains, domain{Key: name, Service: service, Port: port})
			label := "Public domain"
			if service != "" {
				label = "Domain for " + service
			}
			add(field{Key: name, Label: label, Kind: "domain", Group: "network", Service: service, Placeholder: "app.example.com"})
		case strings.HasPrefix(name, "SERVICE_URL_"):
			service, port := resolveService(name, "SERVICE_URL_", services)
			if service == "" {
				continue
			}
			scheme := "http"
			value := fmt.Sprintf("%s://%s", scheme, service)
			if port > 0 {
				value += ":" + strconv.Itoa(port)
			}
			add(field{Key: name, Label: "Internal URL for " + service, Kind: "text", Group: "advanced", Default: value, Service: service, Advanced: true})
		case strings.HasPrefix(name, "SERVICE_PASSWORD_64_"), strings.HasPrefix(name, "SERVICE_PASSWORD_32_"):
			add(field{Key: name, Label: secretLabel(name, "SERVICE_PASSWORD_64_"), Kind: "password", Group: "access", Secret: true, Generate: "hex", Required: true})
		case strings.HasPrefix(name, "SERVICE_PASSWORD_"):
			add(field{Key: name, Label: secretLabel(name, "SERVICE_PASSWORD_"), Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true})
		case strings.HasPrefix(name, "SERVICE_USER_") && accessKeys[name]:
			service, _ := resolveService(name, "SERVICE_USER_", services)
			label := "Access key"
			if service != "" {
				label = displayName(service) + " access key"
			}
			add(field{Key: name, Label: label, Kind: "password", Group: "access", Service: service, Secret: true, Generate: "hex", Required: true})
		case strings.HasPrefix(name, "SERVICE_USER_"):
			service, _ := resolveService(name, "SERVICE_USER_", services)
			value := "admin"
			label := "Username"
			if service != "" {
				value = strings.ReplaceAll(service, "-", "")
				label = displayName(service) + " username"
			}
			add(field{Key: name, Label: label, Kind: "text", Group: "access", Default: value, Service: service, Required: true})
		case strings.HasPrefix(name, "SERVICE_EMAIL_"):
			add(field{Key: name, Label: "Email", Kind: "email", Group: "access", Default: "admin@example.com", Required: true})
		case strings.HasPrefix(name, "SERVICE_BASE64_"), strings.HasPrefix(name, "SERVICE_REALBASE64_"):
			add(field{Key: name, Label: "Generated secret", Kind: "password", Group: "advanced", Secret: true, Generate: "base64", Advanced: true, Required: true})
		case strings.HasPrefix(name, "SERVICE_HEX_"):
			add(field{Key: name, Label: "Generated secret", Kind: "password", Group: "advanced", Secret: true, Generate: "hex", Advanced: true, Required: true})
		default:
			label := humanLabel(name)
			group := fieldGroup(name)
			kind := "text"
			options := []string{}
			switch {
			case strings.EqualFold(fallback, "true") || strings.EqualFold(fallback, "false"):
				kind = "select"
				options = []string{"true", "false"}
			case isNumeric(fallback):
				kind = "number"
			case strings.Contains(strings.ToLower(name), "URL"):
				kind = "url"
			case strings.Contains(strings.ToLower(name), "EMAIL"):
				kind = "email"
			}
			secret := secretVariable.MatchString(name) || strings.Contains(name, "PASSWORD")
			if secret {
				kind = "password"
			}
			generate := ""
			required := reference.required
			if secret && fallback == "" && secretVariable.MatchString(name) {
				generate = "password"
				required = true
			}
			if !required && generate == "" && accessSecretRequired(name) {
				required = true
			}
			add(field{Key: name, Label: label, Kind: kind, Group: group, Default: fallback, Options: options, Secret: secret, Generate: generate, Required: required, Advanced: group == "advanced"})
		}
	}
	return fields, domains
}

func resolveService(variable, prefix string, services map[string]any) (string, int) {
	rest := strings.TrimPrefix(variable, prefix)
	port := 0
	if match := regexp.MustCompile(`^(.*?)_([0-9]{2,5})$`).FindStringSubmatch(rest); match != nil {
		port, _ = strconv.Atoi(match[2])
		rest = match[1]
	}
	normalized := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(rest))
	best := ""
	for name := range services {
		candidate := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(name))
		if candidate == normalized || strings.HasSuffix(normalized, "_"+candidate) || strings.HasSuffix(candidate, "_"+normalized) {
			if len(name) > len(best) {
				best = name
			}
		}
	}
	if best != "" {
		if port == 0 {
			port = servicePort(services[best])
		}
		return best, port
	}
	if port > 0 {
		matches := []string{}
		for name := range services {
			if servicePort(services[name]) == port {
				matches = append(matches, name)
			}
		}
		if len(matches) == 1 {
			return matches[0], port
		}
	}
	if len(services) == 1 {
		for name := range services {
			return name, port
		}
	}
	prefixMatches := []string{}
	for name := range services {
		candidate := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(name))
		if strings.HasPrefix(candidate, normalized) || strings.HasPrefix(normalized, candidate) {
			prefixMatches = append(prefixMatches, name)
		}
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0], port
	}
	return "", port
}

func servicePort(raw any) int {
	svc, _ := raw.(map[string]any)
	if svc == nil {
		return 0
	}
	if ports, ok := svc["ports"].([]any); ok && len(ports) > 0 {
		if target := portTarget(ports[0]); target > 0 {
			return target
		}
	}
	if expose, ok := svc["expose"].([]any); ok && len(expose) > 0 {
		if value, err := strconv.Atoi(strings.TrimSuffix(fmt.Sprint(expose[0]), "/tcp")); err == nil {
			return value
		}
	}
	return 0
}

func portTarget(raw any) int {
	switch value := raw.(type) {
	case string:
		parts := strings.Split(value, ":")
		last := parts[len(parts)-1]
		last = strings.TrimSuffix(last, "/tcp")
		last = strings.TrimSuffix(last, "/udp")
		if port, err := strconv.Atoi(last); err == nil {
			return port
		}
	case map[string]any:
		if target, ok := value["target"].(int); ok {
			return target
		}
		if target, err := strconv.Atoi(fmt.Sprint(value["target"])); err == nil {
			return target
		}
	}
	return 0
}

func pickMain(services map[string]any, fields []field) (string, int) {
	// Prefer a public HTTP port, then a service that looks like the application
	// instead of its database, cache or queue.
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		port := servicePort(services[name])
		if port > 0 && isHTTPPort(port) {
			return name, port
		}
	}
	best, bestScore := "", -100
	for _, name := range names {
		svc, _ := services[name].(map[string]any)
		image, _ := svc["image"].(string)
		score := 0
		role := inferRole(name, image, false)
		switch role {
		case "Database", "Cache", "Queue", "Storage":
			score -= 8
		case "Worker":
			score -= 2
		case "Application", "Service", "Proxy":
			score += 3
		}
		if image != "" {
			score++
		}
		if _, ok := svc["healthcheck"]; ok {
			score += 2
		}
		if _, ok := svc["depends_on"]; ok {
			score += 2
		}
		if environment, ok := svc["environment"].(map[string]any); ok {
			for key := range environment {
				if strings.HasPrefix(key, "SERVICE_FQDN_") || strings.HasPrefix(key, "SERVICE_URL_") {
					score += 3
					break
				}
			}
		}
		if score > bestScore {
			best, bestScore = name, score
		}
	}
	if best != "" {
		return best, servicePort(services[best])
	}
	for _, f := range fields {
		if f.Kind == "domain" && f.Service != "" {
			return f.Service, 80
		}
	}
	return names[0], 0
}

func isHTTPPort(port int) bool {
	switch port {
	case 80, 443, 3000, 3001, 3002, 3003, 4000, 5000, 5601, 8000, 8001, 8080, 8081, 8082, 8083,
		8086, 8088, 8089, 8090, 8123, 8888, 8889, 9000, 9001, 9090, 9091, 9093, 9411, 16686, 3100:
		return true
	}
	return false
}

func serviceInfos(doc map[string]any, mainService string) []serviceInfo {
	services, _ := doc["services"].(map[string]any)
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]serviceInfo, 0, len(names))
	for _, name := range names {
		svc, _ := services[name].(map[string]any)
		image, _ := svc["image"].(string)
		role := inferRole(name, image, name == mainService)
		out = append(out, serviceInfo{Name: name, Image: image, Role: role, Port: servicePort(svc)})
	}
	return out
}

func inferRole(name, image string, main bool) string {
	lower := strings.ToLower(name + " " + image)
	if main {
		return "Application"
	}
	switch {
	case strings.Contains(lower, "postgres"), strings.Contains(lower, "mysql"), strings.Contains(lower, "mariadb"),
		strings.Contains(lower, "mongo"), strings.Contains(lower, "clickhouse"), strings.Contains(lower, "cockroach"),
		strings.Contains(lower, "sqlite"):
		return "Database"
	case strings.Contains(lower, "redis"), strings.Contains(lower, "valkey"), strings.Contains(lower, "memcached"):
		return "Cache"
	case strings.Contains(lower, "rabbitmq"), strings.Contains(lower, "kafka"), strings.Contains(lower, "nats"), strings.Contains(lower, "mqtt"):
		return "Queue"
	case strings.Contains(lower, "minio"), strings.Contains(lower, "garage"), strings.Contains(lower, "seaweed"):
		return "Storage"
	case strings.Contains(lower, "worker"), strings.Contains(lower, "cron"), strings.Contains(lower, "queue"):
		return "Worker"
	case strings.Contains(lower, "nginx"), strings.Contains(lower, "caddy"), strings.Contains(lower, "traefik"):
		return "Proxy"
	}
	return "Service"
}

func walk(value any, fn func(string)) {
	switch typed := value.(type) {
	case string:
		fn(typed)
	case []any:
		for _, item := range typed {
			walk(item, fn)
		}
	case map[string]any:
		for _, item := range typed {
			walk(item, fn)
		}
	}
}

func humanLabel(name string) string {
	words := strings.Split(strings.ToLower(name), "_")
	for i, word := range words {
		if word == "" {
			continue
		}
		if proper, ok := properWords[word]; ok {
			words[i] = proper
			continue
		}
		words[i] = word
	}
	if len(words) > 0 && words[0] != "" {
		if proper, ok := properWords[words[0]]; ok {
			words[0] = proper
		} else {
			words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
		}
	}
	label := strings.Join(words, " ")
	if len(label) > 48 {
		label = label[:48]
	}
	return label
}

var properWords = map[string]string{
	"n8n": "n8n", "sqlite": "SQLite", "postgres": "Postgres", "postgresql": "PostgreSQL",
	"mysql": "MySQL", "mariadb": "MariaDB", "mongodb": "MongoDB", "clickhouse": "ClickHouse",
	"oauth": "OAuth", "openai": "OpenAI", "anthropic": "Anthropic", "ldap": "LDAP",
	"webui": "WebUI", "llm": "LLM", "pdf": "PDF", "url": "URL", "uri": "URI", "api": "API",
	"http": "HTTP", "https": "HTTPS", "smtp": "SMTP", "jwt": "JWT", "oidc": "OIDC",
	"s3": "S3", "tls": "TLS", "ssl": "SSL", "db": "DB", "id": "ID", "cpu": "CPU",
	"ram": "RAM", "aws": "AWS", "gcp": "GCP", "ssh": "SSH", "dns": "DNS", "ip": "IP",
	"tcp": "TCP", "udp": "UDP", "sql": "SQL", "sso": "SSO", "cdn": "CDN", "uuid": "UUID",
}

func secretLabel(name, prefix string) string {
	rest := strings.TrimPrefix(name, prefix)
	switch rest {
	case "APIKEY":
		return "API key"
	case "JWT":
		return "JWT secret"
	case "ENCRYPTIONKEY":
		return "Encryption key"
	case "POSTGRES":
		return "Postgres password"
	case "MYSQL":
		return "MySQL password"
	case "MONGO":
		return "MongoDB password"
	case "REDIS":
		return "Redis password"
	case "CLICKHOUSE":
		return "ClickHouse password"
	case "MINIO":
		return "MinIO password"
	case "N8N":
		return "n8n password"
	}
	rest = strings.TrimSuffix(rest, "KEY")
	label := humanLabel(strings.TrimSpace(rest))
	if label == "" {
		return "Password"
	}
	lower := strings.ToLower(label)
	if !strings.Contains(lower, "password") && !strings.Contains(lower, "secret") && !strings.Contains(lower, "key") {
		label += " password"
	}
	return label
}

func fieldGroup(name string) string {
	upper := strings.ToUpper(name)
	switch {
	case strings.Contains(upper, "PASSWORD"), strings.Contains(upper, "SECRET"), strings.Contains(upper, "TOKEN"),
		strings.Contains(upper, "USER"), strings.Contains(upper, "EMAIL"), strings.Contains(upper, "ADMIN"):
		return "access"
	case strings.Contains(upper, "URL"), strings.Contains(upper, "HOST"), strings.Contains(upper, "DOMAIN"),
		strings.Contains(upper, "PORT"), strings.Contains(upper, "FQDN"):
		return "network"
	}
	return "configuration"
}

func accessSecretRequired(name string) bool {
	upper := strings.ToUpper(name)
	if !strings.Contains(upper, "PASSWORD") && !strings.Contains(upper, "SECRET") {
		return false
	}
	for _, prefix := range []string{"POSTGRES", "POSTGRESQL", "MYSQL", "MARIADB", "MONGO", "MONGODB", "REDIS", "CLICKHOUSE", "MINIO", "MSSQL"} {
		if strings.HasPrefix(upper, prefix) || strings.Contains(upper, "_"+prefix+"_") {
			return true
		}
	}
	return strings.Contains(upper, "ROOT_PASSWORD") || strings.Contains(upper, "ADMIN_PASSWORD") || strings.Contains(upper, "ADMIN_PASS")
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func serviceLogoPath(id, source string) string {
	extension := strings.ToLower(path.Ext(source))
	switch extension {
	case ".png", ".svg", ".webp", ".jpg", ".jpeg", ".gif":
	default:
		extension = ".svg"
	}
	return "/service-logos/" + id + extension
}

func displayName(id string) string {
	known := map[string]string{
		"activepieces": "Activepieces", "actualbudget": "Actual Budget", "affine": "AFFiNE",
		"appflowy": "AppFlowy", "appsmith": "Appsmith", "appwrite": "Appwrite", "authentik": "Authentik",
		"beszel": "Beszel", "calcom": "Cal.com", "chibisafe": "Chibisafe", "clickhouse": "ClickHouse",
		"code-server": "Code Server", "directus": "Directus", "docmost": "Docmost", "dozzle": "Dozzle",
		"esphome": "ESPHome", "firefly": "Firefly III", "gitea": "Gitea", "glance": "Glance",
		"glances": "Glances", "grafana": "Grafana", "homepage": "Homepage", "homarr": "Homarr",
		"immich": "Immich", "it-tools": "IT Tools", "jellyfin": "Jellyfin", "jupyter-notebook-python": "Jupyter Notebook",
		"librechat": "LibreChat", "litellm": "LiteLLM", "mattermost": "Mattermost", "metabase": "Metabase",
		"minecraft": "Minecraft", "mixpost": "Mixpost", "n8n": "n8n", "navidrome": "Navidrome",
		"nextcloud": "Nextcloud", "nocodb": "NocoDB", "openpanel": "OpenPanel", "opnform": "OpnForm",
		"outline": "Outline", "paperless": "Paperless-ngx", "plane": "Plane", "portainer": "Portainer",
		"pterodactyl-panel": "Pterodactyl Panel", "rocketchat": "Rocket.Chat", "searxng": "SearXNG",
		"signoz": "SigNoz", "stalwart": "Stalwart", "supabase": "Supabase", "superset-with-postgresql": "Superset",
		"syncthing": "Syncthing", "traccar": "Traccar", "uptime-kuma": "Uptime Kuma", "vaultwarden": "Vaultwarden",
		"wordpress": "WordPress", "9router": "9Router",
		"anythingllm": "AnythingLLM", "apprise-api": "Apprise API", "mage-ai": "Mage AI",
		"open-webui": "Open WebUI", "pocket-id": "Pocket ID", "pocket-id-with-postgresql": "Pocket ID",
		"uptime-kuma-with-mariadb": "Uptime Kuma", "uptime-kuma-with-mysql": "Uptime Kuma",
		"hermes-agent": "Hermes Agent", "hermes-agent-with-webui": "Hermes Agent",
		"ollama-with-open-webui": "Ollama with Open WebUI", "bento-pdf": "BentoPDF",
		"calibre-web-automated-book-downloader": "Calibre Web Automated", "glitchtip": "GlitchTip",
		"linkding-plus": "Linkding", "neon-ws-proxy": "Neon WS Proxy", "reactive-resume": "Reactive Resume",
		"rivet-engine": "Rivet Engine", "sparkyfitness": "SparkyFitness", "stirling-pdf": "Stirling PDF",
		"web-check": "Web Check", "hoppscotch": "Hoppscotch", "drizzle-gateway": "Drizzle Gateway",
	}
	if value, ok := known[id]; ok {
		return value
	}
	parts := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func displayCategory(value string) string {
	if value == "" {
		return ""
	}
	known := map[string]string{
		"ai": "AI", "cms": "CMS", "database": "Databases", "rss": "RSS", "api": "API",
		"sql": "SQL", "voip": "VoIP", "dns": "DNS", "sso": "SSO", "git": "Git",
		"devtools": "Devtools", "media": "Media", "messaging": "Messaging",
	}
	if label, ok := known[strings.ToLower(value)]; ok {
		return label
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' })
	for i, part := range parts {
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func logomark(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) == 0 {
		return "?"
	}
	if len(words) == 1 {
		runes := []rune(strings.ToUpper(words[0]))
		if len(runes) == 1 {
			return string(runes)
		}
		return string(runes[:2])
	}
	first := []rune(strings.ToUpper(words[0]))
	second := []rune(strings.ToUpper(words[1]))
	return string(first[0]) + string(second[0])
}

func cleanTags(tags []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" || len(tag) > 32 || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func cleanURL(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "?utm_source=coolify.io", "")
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return ""
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func humanJoin(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " and " + values[1]
	}
	return strings.Join(values[:len(values)-1], ", ") + " and " + values[len(values)-1]
}
