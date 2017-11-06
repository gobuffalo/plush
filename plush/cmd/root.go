package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"encoding/json"
	"strings"

	"github.com/gobuffalo/plush"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ctxFile *string
var vals *[]string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "plush",
	Short: "A brief description of your application",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("you must pass in at least 1 plush file")
		}

		for _, a := range args {
			b, err := ioutil.ReadFile(a)
			if err != nil {
				return errors.WithStack(err)
			}
			err = plush.RunScript(string(b), plush.NewContext())
			if err != nil {
				return errors.WithStack(err)
			}

		}
		return nil
	},
}

// parseContextVars accepts CLI input of form -v x=y -v a=b and
// returns a Hash of form {x:y, a:b} which can be loaded into a context.
func parseContextVars(values []string, out map[string]interface{}) error {
	if len(values) == 0 {
		return nil
	}

	for _, x := range values {
		parts := strings.SplitN(x, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}

	return nil
}

// parseContextFile reads File from location, parses it into string, interface map.
// Returns error on every other failing condition.
func parseContextFile(loc string, out map[string]interface{}) error {
	if loc == "" {
		return nil
	}

	ctxBytes, err := ioutil.ReadFile(loc)
	if err != nil {
		return errors.WithStack(err)
	}

	return parseContextBytes(ctxBytes, out)
}

// Internal function to parseContextFile.
func parseContextBytes(b []byte, out map[string]interface{}) error {
	if err := json.Unmarshal(b, &out); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Internal function of RenderCmd
func renderTmpl(tmpFile string, ctxFile string, vals []string) (string, error) {
	bytes, err := ioutil.ReadFile(tmpFile)
	if err != nil {
		return "", errors.WithStack(err)
	}

	vars := map[string]interface{}{}
	if err := parseContextFile(ctxFile, vars); err != nil {
		return "", errors.WithStack(err)
	}

	if err := parseContextVars(vals, vars); err != nil {
		return "", errors.WithStack(err)
	}

	ctx := plush.NewContextWith(vars)
	out, err := plush.Render(string(bytes), ctx)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return out, nil
}

// parseCmd renders a plush template file with the vars provided either as switches or an input file
var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render a plush template using Command line Input or a JSON input to build context",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("Must provide a plush template file")
		}

		out, err := renderTmpl(args[0], *ctxFile, *vals)
		if err != nil {
			return err
		}

		fmt.Println(out)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ctxFile = renderCmd.PersistentFlags().StringP("contextFile", "c", "", "File to generate context from.")
	vals = renderCmd.PersistentFlags().StringArrayP("contextVal", "v", []string{}, "Key=Value to be rendered.")
	RootCmd.AddCommand(renderCmd)
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
