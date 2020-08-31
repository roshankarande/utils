package helper

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
	"os/exec"
	"time"
)

func ExecutePowershellCmd(command string, environment map[string]string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	ps, _ := exec.LookPath("powershell.exe")

	var env []string

	for k, v := range environment {
		env = append(env,fmt.Sprintf("%s=%s",k,v))
	}

	cmd := exec.Cmd{
		Path:   ps,
		Args:   []string{ "-NoProfile", "-NonInteractive","-Command", command },
		Env:    env,
		Dir:    "",
		Stdout: &stdout,
		Stderr: &stderr,
	}
	err := cmd.Run()

	return stdout.String(), stderr.String(), err
}

// key is the command key e.g. command_before_create
func ExecuteCommand(d *schema.ResourceData, key string, marshalFunc func (d *schema.ResourceData)(string, error)) error {
	cmd := d.Get(key).(string)

	if cmd == "" {
		return nil
	}

	JsonSchema, err := marshalFunc(d)
	if err != nil {
		return err
	}

	m := map[string]string{
		"ResourceData" : JsonSchema,
		"StartTime"  : time.Now().UTC().Format(time.RFC3339),
	}

	log.Printf("[DEBUG] : __custom__ : [executing]%s - [cmd]%s - [env]%v ",key, cmd, m)

	stdout, stderr, err := ExecutePowershellCmd(cmd, m)

	log.Printf("[DEBUG] : __custom__ : [result]%s - [cmd]%s - [stdout]%s, [stderr]%s, [err]%v ",key, cmd, stdout, stderr, err)

	if err != nil || stderr != "" {
		log.Printf("[DEBUG] : __custom__ : error executing command - %s [err]%v  [stderr]%v ", cmd, err, stderr)
		return fmt.Errorf("error executing command - %s [err]%v  [stderr]%v ", cmd, err, stderr)
	}

	log.Printf("[DEBUG] : __custom__ : %s executed Succesfully", key)

	return nil
}
