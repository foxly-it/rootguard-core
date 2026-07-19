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
		AdGuard: inspectContainer("rootguard-adguard"),
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

	var data []struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
		NetworkSettings struct {
			Ports map[string]json.RawMessage `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		return ContainerInfo{}
	}

	if len(data) == 0 {
		return ContainerInfo{
			Exists:  false,
			Running: false,
		}
	}

	var ports []string
	for port := range data[0].NetworkSettings.Ports {
		ports = append(ports, port)
	}

	return ContainerInfo{
		Exists:  true,
		Running: data[0].State.Running,
		Image:   data[0].Config.Image,
		Ports:   ports,
	}
}
