package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

// getEditor finds the path to a editor to use for file editing.
func getEditor() string {
	userSpecifiedEditor := os.Getenv("EDITOR")
	if userSpecifiedEditor != "" {
		return userSpecifiedEditor
	}

	// If no editor env is specified, try vim followed by nano.
	// This is because vi >> nano :)
	path, err := exec.LookPath("vim")
	if err == nil {
		return path
	}

	path, err = exec.LookPath("nano")
	if err != nil {
		return ""
	}

	return path
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	hasher := md5.New()
	if _, err = io.Copy(hasher, f); err != nil {
		return "", err
	}

	out := []byte{}
	if _, err = hasher.Write(out); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func newEditCommand(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "edit <service>",
		Short:        "Opens a service configuration in your default editor and deploys it on saving.",
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		engine, cfg, err := eP()
		if err != nil {
			return err
		}
		serviceName := args[0]

		editorPath := getEditor()
		if editorPath == "" {
			return errors.New(
				"unable to find default editor, try configuring $EDITOR",
			)
		}

		servicePath := path.Join(cfg.Paths.Config, serviceName+".yaml")

		// Hash file so we can tell if the user makes any changes, we don't want
		// to deploy if they haven't.
		before, err := hashFile(servicePath)
		if err != nil {
			return err
		}

		editorCmd := exec.Command(
			editorPath, servicePath,
		)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr

		_, err = infoColour.Fprintf(
			cmd.OutOrStdout(),
			"✍️  Opening '%s' for editing.\n",
			servicePath,
		)
		if err != nil {
			return err
		}

		if err = editorCmd.Run(); err != nil {
			return err
		}

		after, err := hashFile(servicePath)
		if err != nil {
			return err
		}

		if before == after {
			_, err = infoColour.Fprintln(
				cmd.OutOrStdout(),
				"🤷 No changes made to file, not deploying.",
			)
			return err
		}

		_, err = infoColour.Fprintf(
			cmd.OutOrStdout(),
			"🔨 Deploying '%s'. Hold on tight!\n",
			serviceName,
		)
		if err != nil {
			return err
		}

		res, err := engine.Deploy(cmd.Context(), guvnor.DeployArgs{
			ServiceName: serviceName,
		})
		if err != nil {
			return err
		}

		_, err = successColour.Fprintf(
			cmd.OutOrStdout(),
			"✅ Succesfully deployed '%s'. Deployment ID is %d.\n",
			res.ServiceName,
			res.DeploymentID,
		)
		return err
	}

	return cmd
}
