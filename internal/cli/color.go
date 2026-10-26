package cli

import (
	"io"

	"github.com/jmcampanini/grove/internal/render"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const colorFlag = "color"

// colorValue is the pflag value behind the persistent --color flag. Set
// rejects anything outside auto, always, and never, so an unknown mode fails
// flag parsing as a usage error that names the value.
type colorValue struct {
	mode render.ColorMode
}

func (v *colorValue) String() string { return string(v.mode) }

func (v *colorValue) Set(s string) error {
	mode, err := render.ParseColorMode(s)
	if err != nil {
		return err
	}
	v.mode = mode
	return nil
}

func (*colorValue) Type() string { return "mode" }

func registerColorFlag(flags *pflag.FlagSet) {
	flags.Var(&colorValue{mode: render.ColorAuto}, colorFlag, "Color: auto, always, never")
}

// colorMode returns the --color mode of the execution cmd belongs to. A tree
// without the flag (a bare test command) runs in auto.
func colorMode(cmd *cobra.Command) render.ColorMode {
	flag := cmd.Root().PersistentFlags().Lookup(colorFlag)
	if flag == nil {
		return render.ColorAuto
	}
	return flag.Value.(*colorValue).mode
}

// styledStdout returns the command's stdout wrapped for styled output under
// the execution's --color mode. Machine-readable payloads never go through
// it.
func styledStdout(cmd *cobra.Command) io.Writer {
	return render.StyledWriter(colorMode(cmd), cmd.OutOrStdout())
}
