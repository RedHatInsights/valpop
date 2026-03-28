package cmd

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var _ = Describe("Command flag conflicts", func() {
	It("should not have conflicting flag names or shorthands across commands", func() {
		type flagDef struct {
			commandPath string
			flagName    string
			shorthand   string
			scope       string
		}

		declaredNames := map[string]flagDef{}
		declaredShorthands := map[string]flagDef{}
		conflicts := []string{}

		register := func(commandPath, scope string, flag *pflag.Flag) {
			current := flagDef{
				commandPath: commandPath,
				flagName:    flag.Name,
				shorthand:   flag.Shorthand,
				scope:       scope,
			}

			if existing, exists := declaredNames[flag.Name]; exists {
				conflicts = append(conflicts, fmt.Sprintf(
					"flag name --%s declared twice: %s (%s) and %s (%s)",
					flag.Name,
					existing.commandPath,
					existing.scope,
					current.commandPath,
					current.scope,
				))
			} else {
				declaredNames[flag.Name] = current
			}

			if flag.Shorthand == "" {
				return
			}

			if existing, exists := declaredShorthands[flag.Shorthand]; exists {
				conflicts = append(conflicts, fmt.Sprintf(
					"shorthand -%s declared twice: --%s on %s (%s) and --%s on %s (%s)",
					flag.Shorthand,
					existing.flagName,
					existing.commandPath,
					existing.scope,
					current.flagName,
					current.commandPath,
					current.scope,
				))
			} else {
				declaredShorthands[flag.Shorthand] = current
			}
		}

		queue := []*cobra.Command{rootCmd}
		for len(queue) > 0 {
			cmd := queue[0]
			queue = queue[1:]

			cmdPath := cmd.CommandPath()

			cmd.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
				register(cmdPath, "local", flag)
			})
			cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
				register(cmdPath, "persistent", flag)
			})

			queue = append(queue, cmd.Commands()...)
		}

		Expect(conflicts).To(BeEmpty(), strings.Join(conflicts, "\n"))
	})
})
