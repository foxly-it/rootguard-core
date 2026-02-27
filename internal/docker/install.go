package docker

import (
	"bufio"
	"fmt"
	"os/exec"
)

func InstallDockerAsync() {

	go func() {

		setRunning()

		appendLog("Starting Docker installation...")

		steps := []string{
			"apt update",
			"apt install -y ca-certificates curl gnupg",
			"install -m 0755 -d /etc/apt/keyrings",
			"curl -fsSL https://download.docker.com/linux/debian/gpg | tee /etc/apt/keyrings/docker.asc > /dev/null",
			"chmod a+r /etc/apt/keyrings/docker.asc",
			`echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo $VERSION_CODENAME) stable" > /etc/apt/sources.list.d/docker.list`,
			"apt update",
			"apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
			"systemctl enable docker",
			"systemctl start docker",
		}

		for _, command := range steps {

			appendLog("Running: " + command)

			cmd := exec.Command("bash", "-c", command)

			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			if err := cmd.Start(); err != nil {
				setDone(err)
				return
			}

			scannerOut := bufio.NewScanner(stdout)
			scannerErr := bufio.NewScanner(stderr)

			for scannerOut.Scan() {
				appendLog(scannerOut.Text())
			}

			for scannerErr.Scan() {
				appendLog(scannerErr.Text())
			}

			if err := cmd.Wait(); err != nil {
				appendLog(fmt.Sprintf("ERROR: %v", err))
				setDone(err)
				return
			}
		}

		appendLog("Docker installation completed successfully.")
		setDone(nil)
	}()
}
