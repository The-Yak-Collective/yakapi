package ci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func Accept(ctx context.Context, cmd string) error {
	if cmd == "" {
		return errors.New("empty command")
	}

	f := strings.Fields(cmd)
	switch f[0] {
	case "ping":
		return nil
	case "fwd":
		return doFwd(ctx, f[1:])
	}

	return nil
}

func doFwd(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	durationArg, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse duration: %w", err)
	}

	duration := time.Duration(durationArg) * time.Millisecond * 10

	err = execMotorAdapter(ctx, []string{"motorA:0.75", "motorB:0.75"})
	if err != nil {
		return err
	}

	fmt.Printf("sleeping for %.3fs\n", duration.Seconds())
	time.Sleep(duration)

	err = execMotorAdapter(ctx, []string{"motorA:0.0", "motorB:0.0"})
	if err != nil {
		return err
	}

	return nil
}

func execMotorAdapter(ctx context.Context, args []string) error {
	name := os.Getenv("YAKAPI_ADAPTER_MOTOR")
	if name == "" {
		return errors.New("motor adapter not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed running motor adapter: %w", err)
	}

	return nil
}
