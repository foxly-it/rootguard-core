package stack

import (
	"fmt"
	"os"
	"os/exec"
)

const stackPath = "/home/rootguard/stack"
const composeFile = "/home/rootguard/stack/docker-compose.yml"

func DeployStack() error {

	// Ordnerstruktur sicherstellen
	if err := os.MkdirAll(stackPath+"/adguard", 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(stackPath+"/unbound", 0755); err != nil {
		return err
	}

	// Compose Datei generieren
	composeContent := generateCompose()

	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		return err
	}

	// Deploy ausführen
	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func generateCompose() string {

	return fmt.Sprintf(`
services:

  rootguard-adguard:
    container_name: rootguard-adguard
    image: adguard/adguardhome:latest
    restart: unless-stopped
    ports:
      - "53:53/tcp"
      - "53:53/udp"
      - "3000:3000"
    volumes:
      - ./adguard/work:/opt/adguardhome/work
      - ./adguard/conf:/opt/adguardhome/conf

  rootguard-unbound:
    container_name: rootguard-unbound
    image: ghcr.io/foxly-it/rootguard-unbound:latest
    restart: unless-stopped

    # 🔒 Hardening Level 2
    read_only: true

    cap_drop:
      - ALL

    cap_add:
      - NET_BIND_SERVICE

    security_opt:
      - no-new-privileges:true

    tmpfs:
      - /tmp
      - /var/lib/unbound

    volumes:
      - ./unbound:/config

    # Kein Portmapping – nur intern über AdGuard erreichbar
    network_mode: "service:rootguard-adguard"
`)
}
