package nativets

import (
	"os"
	"os/exec"
)

func YogaLayout() error {
	return nil
}

func ColorDiff() error {
	return nil
}

func FileIndex() error {
	return nil
}

func RunNative(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
