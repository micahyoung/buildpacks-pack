package build

import (
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types"

	"github.com/buildpacks/pack/internal/style"
	"github.com/buildpacks/pack/logging"
)

const (
	linuxContainerAdmin   = "root"
	windowsContainerAdmin = `NT AUTHORITY\SYSTEM`
	platformAPIEnvVar     = "CNB_PLATFORM_API"
)

type PhaseConfigProviderOperation func(*PhaseConfigProvider)

type PhaseConfigProvider struct {
	ctrConf      *container.Config
	ctrExecs     []*types.ExecConfig
	hostConf     *container.HostConfig
	name         string
	os           string
	containerOps []ContainerOperation
	infoWriter   io.Writer
	errorWriter  io.Writer
}

func NewPhaseConfigProvider(name string, lifecycleExec *LifecycleExecution, ops ...PhaseConfigProviderOperation) *PhaseConfigProvider {
	provider := &PhaseConfigProvider{
		ctrConf:     new(container.Config),
		hostConf:    new(container.HostConfig),
		name:        name,
		os:          lifecycleExec.os,
		infoWriter:  logging.GetWriterForLevel(lifecycleExec.logger, logging.InfoLevel),
		errorWriter: logging.GetWriterForLevel(lifecycleExec.logger, logging.ErrorLevel),
	}

	provider.ctrConf.Image = lifecycleExec.opts.Builder.Name()
	provider.ctrConf.Labels = map[string]string{"author": "pack"}

	if lifecycleExec.os == "windows" {
		provider.hostConf.Isolation = container.IsolationProcess
	}

	ops = append(ops,
		WithEnv(fmt.Sprintf("%s=%s", platformAPIEnvVar, lifecycleExec.platformAPI.String())),
		WithLifecycleProxy(lifecycleExec),
		WithBinds([]string{
			fmt.Sprintf("%s:%s", lifecycleExec.layersVolume, lifecycleExec.mountPaths.layersDir()),
			fmt.Sprintf("%s:%s", lifecycleExec.appVolume, lifecycleExec.mountPaths.appDir()),
		}...),
	)

	for _, op := range ops {
		op(provider)
	}

	provider.ctrConf.Cmd = append([]string{"/cnb/lifecycle/" + name}, provider.ctrConf.Cmd...)

	lifecycleExec.logger.Debugf("Running the %s on OS %s with:", style.Symbol(provider.Name()), style.Symbol(provider.os))
	lifecycleExec.logger.Debug("Container Settings:")
	lifecycleExec.logger.Debugf("  Args: %s", style.Symbol(strings.Join(provider.ctrConf.Cmd, " ")))
	lifecycleExec.logger.Debugf("  System Envs: %s", style.Symbol(strings.Join(provider.ctrConf.Env, " ")))
	lifecycleExec.logger.Debugf("  Image: %s", style.Symbol(provider.ctrConf.Image))
	lifecycleExec.logger.Debugf("  User: %s", style.Symbol(provider.ctrConf.User))
	lifecycleExec.logger.Debugf("  Labels: %s", style.Symbol(fmt.Sprintf("%s", provider.ctrConf.Labels)))

	lifecycleExec.logger.Debug("Host Settings:")
	lifecycleExec.logger.Debugf("  Binds: %s", style.Symbol(strings.Join(provider.hostConf.Binds, " ")))
	lifecycleExec.logger.Debugf("  Network Mode: %s", style.Symbol(string(provider.hostConf.NetworkMode)))
	return provider
}

func (p *PhaseConfigProvider) ContainerConfig() *container.Config {
	return p.ctrConf
}

func (p *PhaseConfigProvider) ContainerOps() []ContainerOperation {
	return p.containerOps
}

func (p *PhaseConfigProvider) HostConfig() *container.HostConfig {
	return p.hostConf
}

func (p *PhaseConfigProvider) Name() string {
	return p.name
}

func (p *PhaseConfigProvider) ErrorWriter() io.Writer {
	return p.errorWriter
}

func (p *PhaseConfigProvider) InfoWriter() io.Writer {
	return p.infoWriter
}

func NullOp() PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {}
}

func WithArgs(args ...string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.ctrConf.Cmd = append(provider.ctrConf.Cmd, args...)
	}
}

// WithFlags differs from WithArgs as flags are always prepended
func WithFlags(flags ...string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.ctrConf.Cmd = append(flags, provider.ctrConf.Cmd...)
	}
}

func WithBinds(binds ...string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.hostConf.Binds = append(provider.hostConf.Binds, binds...)
	}
}

func WithDaemonAccess() PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		WithRoot()(provider)
		bind := "/var/run/docker.sock:/var/run/docker.sock"
		if provider.os == "windows" {
			bind = `\\.\pipe\docker_engine:\\.\pipe\docker_engine`
		}
		provider.hostConf.Binds = append(provider.hostConf.Binds, bind)
	}
}

func WithEnv(envs ...string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.ctrConf.Env = append(provider.ctrConf.Env, envs...)
	}
}

func WithImage(image string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.ctrConf.Image = image
	}
}

// WithLogPrefix sets a prefix for logs produced by this phase
func WithLogPrefix(prefix string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		if prefix != "" {
			provider.infoWriter = logging.NewPrefixWriter(provider.infoWriter, prefix)
			provider.errorWriter = logging.NewPrefixWriter(provider.errorWriter, prefix)
		}
	}
}

func WithLifecycleProxy(lifecycleExec *LifecycleExecution) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		if lifecycleExec.opts.HTTPProxy != "" {
			provider.ctrConf.Env = append(provider.ctrConf.Env, "HTTP_PROXY="+lifecycleExec.opts.HTTPProxy)
			provider.ctrConf.Env = append(provider.ctrConf.Env, "http_proxy="+lifecycleExec.opts.HTTPProxy)
		}

		if lifecycleExec.opts.HTTPSProxy != "" {
			provider.ctrConf.Env = append(provider.ctrConf.Env, "HTTPS_PROXY="+lifecycleExec.opts.HTTPSProxy)
			provider.ctrConf.Env = append(provider.ctrConf.Env, "https_proxy="+lifecycleExec.opts.HTTPSProxy)
		}

		if lifecycleExec.opts.NoProxy != "" {
			provider.ctrConf.Env = append(provider.ctrConf.Env, "NO_PROXY="+lifecycleExec.opts.NoProxy)
			provider.ctrConf.Env = append(provider.ctrConf.Env, "no_proxy="+lifecycleExec.opts.NoProxy)
		}
	}
}

func WithNetwork(networkMode string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.hostConf.NetworkMode = container.NetworkMode(networkMode)
	}
}

func WithRegistryAccess(authConfig string) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.ctrConf.Env = append(provider.ctrConf.Env, fmt.Sprintf(`CNB_REGISTRY_AUTH=%s`, authConfig))
	}
}

func WithRoot() PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		if provider.os == "windows" {
			provider.ctrConf.User = windowsContainerAdmin

			// exec process as default user than can be impersonated by SYSTEM user
			// run cmd in the background to prompt for input forever
			provider.ctrExecs = []*types.ExecConfig{{
				Cmd:          []string{"cmd.exe", "/c", "set /p wait="},
				Detach:       true,
				AttachStdin:  true,
				User:         "",
			}}
		} else {
			provider.ctrConf.User = linuxContainerAdmin
		}
	}
}

func WithContainerOperations(operations ...ContainerOperation) PhaseConfigProviderOperation {
	return func(provider *PhaseConfigProvider) {
		provider.containerOps = append(provider.containerOps, operations...)
	}
}
