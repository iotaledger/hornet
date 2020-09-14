package framework

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
)

// getWebAPIBaseURL returns the web API base url for the given IP.
func getWebAPIBaseURL(hostname string) string {
	return fmt.Sprintf("http://%s:%d", hostname, WebAPIPort)
}

// returns the given error if the provided context is done.
func returnErrIfCtxDone(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		return err
	default:
		return nil
	}
}

// consumes the given read closer by piping its content to a file in the log directory by the given name.
func writeFileInLogDir(name string, readCloser io.ReadCloser) error {
	defer readCloser.Close()

	f, err := os.Create(fmt.Sprintf("%s%s", logsDir, name))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, readCloser); err != nil {
		return fmt.Errorf("couldn't copy content: %w", err)
	}

	return nil
}

// createContainerLogFile creates a log file from the given ReadCloser.
// it strips away non ascii chars from the given ReadCloser.
func createContainerLogFile(name string, logs io.ReadCloser) error {
	defer logs.Close()

	f, err := os.Create(fmt.Sprintf("%s%s.log", logsDir, name))
	if err != nil {
		return err
	}
	defer f.Close()

	// remove non-ascii chars at beginning of line
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Bytes()

		// in case of an error there is no Docker prefix
		var bytes []byte
		if len(line) < dockerLogsPrefixLen {
			bytes = append(line, '\n')
		} else {
			bytes = append(line[dockerLogsPrefixLen:], '\n')
		}

		_, err = f.Write(bytes)
		if err != nil {
			return err
		}
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	return nil
}
