package devcontainers

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Start spins up postgres and minio dev containers if they aren't already
// running. Containers are named and reused across restarts so data persists.
// To wipe: docker rm -f agentra-dev-postgres agentra-dev-minio
func Start(ctx context.Context) error {
	// Disable Ryuk so named containers survive process exit
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	if err := startPostgres(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := startMinio(ctx); err != nil {
		return fmt.Errorf("minio: %w", err)
	}
	return nil
}

func startPostgres(ctx context.Context) error {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "pgvector/pgvector:pg16",
			Name:  "agentra-dev-postgres",
			Env: map[string]string{
				"POSTGRES_DB":       "agentra",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "postgres",
			},
			ExposedPorts: []string{"5432/tcp"},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.PortBindings = nat.PortMap{
					"5432/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "5432"}},
				}
			},
			WaitingFor: wait.ForListeningPort("5432/tcp"),
		},
		Started: true,
		Reuse:   true,
	})
	if err != nil {
		return err
	}

	host, err := c.Host(ctx)
	if err != nil {
		return err
	}
	port, err := c.MappedPort(ctx, "5432")
	if err != nil {
		return err
	}

	dsn := fmt.Sprintf("postgres://postgres:postgres@%s:%s/agentra?sslmode=disable", host, port.Port())
	os.Setenv("DATABASE_URL", dsn)
	fmt.Printf("[devcontainers] postgres -> %s\n", dsn)
	return nil
}

func startMinio(ctx context.Context) error {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "minio/minio:latest",
			Name:  "agentra-dev-minio",
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd:          []string{"server", "/data", "--console-address", ":9001"},
			ExposedPorts: []string{"9000/tcp", "9001/tcp"},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.PortBindings = nat.PortMap{
					"9000/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9000"}},
					"9001/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9001"}},
				}
			},
			WaitingFor: wait.ForListeningPort("9000/tcp"),
		},
		Started: true,
		Reuse:   true,
	})
	if err != nil {
		return err
	}

	host, err := c.Host(ctx)
	if err != nil {
		return err
	}
	port, err := c.MappedPort(ctx, "9000")
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())
	os.Setenv("MINIO_ENDPOINT", endpoint)
	os.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	os.Setenv("MINIO_SECRET_KEY", "minioadmin")
	os.Setenv("MINIO_BUCKET", "agentra-media")
	os.Setenv("MINIO_BASE_PATH", "media")
	os.Setenv("MINIO_OUTPUT_BASE_PATH", "renders")
	fmt.Printf("[devcontainers] minio    -> %s (console: http://%s:%s)\n", endpoint, host, func() string {
		p, _ := c.MappedPort(ctx, "9001")
		return p.Port()
	}())
	return nil
}
