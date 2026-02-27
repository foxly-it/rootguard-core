package stack

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

type ContainerInfo struct {
	Exists  bool     `json:"exists"`
	Running bool     `json:"running"`
	Image   string   `json:"image,omitempty"`
	Ports   []string `json:"ports,omitempty"`
}

type StackStatus struct {
	AdGuard ContainerInfo `json:"adguard"`
	Unbound ContainerInfo `json:"unbound"`
}

func CheckStackStatus() StackStatus {

	return StackStatus{
		AdGuard: inspectContainer("adguardhome"),
		Unbound: inspectContainer("rootguard-unbound"),
	}
}

func inspectContainer(name string) ContainerInfo {

	cmd := exec.Command("docker", "inspect", name)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return ContainerInfo{
			Exists:  false,
			Running: false,
		}
	}

	var data []map[string]interface{}
	json.Unmarshal(out.Bytes(), &data)

	if len(data) == 0 {
		return ContainerInfo{
			Exists:  false,
			Running: false,
		}
	}

	state := data[0]["State"].(map[string]interface{})
	config := data[0]["Config"].(map[string]interface{})
	network := data[0]["NetworkSettings"].(map[string]interface{})

	image := config["Image"].(string)
	running := state["Running"].(bool)

	var ports []string

	if network["Ports"] != nil {
		for k := range network["Ports"].(map[string]interface{}) {
			ports = append(ports, k)
		}
	}

	return ContainerInfo{
		Exists:  true,
		Running: running,
		Image:   image,
		Ports:   ports,
	}
}
