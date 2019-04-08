package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/buildpack/pack"
	"github.com/buildpack/pack/config"
	"github.com/buildpack/pack/logging"
	"github.com/buildpack/pack/style"
)

func CreateBuilder(logger *logging.Logger, fetcher pack.Fetcher, bpFetcher pack.BuildpackFetcher) *cobra.Command {
	var flags pack.CreateBuilderFlags
	ctx := createCancellableContext()
	cmd := &cobra.Command{
		Use:   "create-builder <image-name> --builder-config <builder-config-path>",
		Args:  cobra.ExactArgs(1),
		Short: "Create builder image",
		RunE: logError(logger, func(cmd *cobra.Command, args []string) error {
			flags.RepoName = args[0]
			// if runtime.GOOS == "windows" {
			// 	return fmt.Errorf("%s is not implemented on Windows", style.Symbol("create-builder"))
			// }

			cfg, err := config.NewDefault()
			if err != nil {
				return err
			}
			builderFactory := pack.BuilderFactory{
				Logger:           logger,
				Config:           cfg,
				Fetcher:          fetcher,
				BuildpackFetcher: bpFetcher,
			}
			builderConfig, err := builderFactory.BuilderConfigFromFlags(ctx, flags)
			if err != nil {
				return err
			}
			if err := builderFactory.Create(builderConfig); err != nil {
				return err
			}
			imageName := builderConfig.Repo.Name()
			logger.Info("Successfully created builder image %s", style.Symbol(imageName))
			logger.Tip("Run %s to use this builder", style.Symbol(fmt.Sprintf("pack build <image-name> --builder %s", imageName)))
			return nil
		}),
	}
	cmd.Flags().BoolVar(&flags.NoPull, "no-pull", false, "Skip pulling build image before use")
	cmd.Flags().StringVarP(&flags.BuilderTomlPath, "builder-config", "b", "", "Path to builder TOML file (required)")
	cmd.MarkFlagRequired("builder-config")
	cmd.Flags().BoolVar(&flags.Publish, "publish", false, "Publish to registry")
	AddHelpFlag(cmd, "create-builder")
	return cmd
}
