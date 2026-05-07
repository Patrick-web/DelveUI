package adapter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Install runs spec.InstallCmd via /bin/sh -c, calling onOutput for each
// line of combined stdout/stderr. After the command completes, it calls
// reg.Rediscover(language) to verify the binary was installed. Returns
// nil on success (binary found after install).
func Install(ctx context.Context, reg *Registry, spec ProcessSpec, onOutput func(string)) error {
	if spec.InstallCmd == "" {
		return fmt.Errorf("no install command for %s", spec.Language)
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", spec.InstallCmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start install: %w", err)
	}

	// Read stdout and stderr concurrently, forwarding each line to onOutput.
	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		onOutput(scanner.Text())
	}

	waitErr := cmd.Wait()

	// Re-discover the binary after install attempt.
	reg.Rediscover(spec.Language)
	if !reg.Installed(spec.Language) {
		if waitErr != nil {
			return fmt.Errorf("install failed and binary not found (%v)", waitErr)
		}
		return fmt.Errorf("install completed but %q binary not found — run: %s", spec.BinaryName, spec.InstallCmd)
	}

	return nil
}
