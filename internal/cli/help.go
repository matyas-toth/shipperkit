package cli

import (
	"fmt"
	"strings"
)

func (c *Client) help(command string) error {
	usage := map[string]string{
		"connect":  "connect URL --token-stdin\n  Read a scoped token from stdin or SHIPPER_TOKEN and save the verified connection.",
		"apps":     "apps [--json]",
		"inspect":  "inspect APP [--json]",
		"doctor":   "doctor APP [--json]",
		"plan":     "plan [DIRECTORY] [--app APP|--new] [--name NAME] [--method auto|static|dockerfile|railpack|compose] [--config FILE] [--domain HOST] [--port PORT] [--worker] [--json]",
		"deploy":   "deploy [DIRECTORY] [--app APP|--new] [--name NAME] [--method METHOD] [--config FILE] [--domain HOST] [--port PORT] [--worker] [--wait] [--json]\n  deploy --plan PLAN_ID [--wait] [--json]",
		"watch":    "watch APP --operation ID [--timeout 10m] [--json]",
		"logs":     "logs APP [--operation ID --after CURSOR] [--service NAME] [--json]",
		"rollback": "rollback APP --operation RELEASE_ID [--apply --revision REVIEWED_REVISION] [--json]",
		"task":     "task APP TASK_ID [--json]",
	}
	value, ok := usage[command]
	if !ok {
		return fmt.Errorf("Unknown command %s", command)
	}
	if c.JSON {
		return c.output(map[string]string{"command": command, "usage": "shipper " + value})
	}
	_, e := fmt.Fprintln(c.Out, "Usage: shipper "+strings.TrimSpace(value))
	return e
}
