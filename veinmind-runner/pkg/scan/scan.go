package scan

import (
	"context"
	"path"

	api "github.com/chaitin/libveinmind/go"
	"github.com/chaitin/libveinmind/go/cmd"
	"github.com/chaitin/libveinmind/go/docker"
	"github.com/chaitin/libveinmind/go/plugin"
	"github.com/chaitin/libveinmind/go/plugin/log"
	"github.com/chaitin/libveinmind/go/plugin/service"
	"github.com/chaitin/libveinmind/go/plugin/specflags"
	"github.com/chaitin/veinmind-common-go/service/report"
)

func FindTargetPlugins(ctx context.Context, enablePlugins []string) ([]*plugin.Plugin, error) {
	ps, err := plugin.DiscoverPlugins(ctx, ".")
	if err != nil {
		return nil, err
	}
	pluginMap := make(map[string]*plugin.Plugin)
	for _, p := range ps {
		pluginMap[p.Name] = p
	}
	// find the intersection of plugins
	// between found in runner and user specified
	finalPs := []*plugin.Plugin{}
	for _, item := range enablePlugins {
		if p, ok := pluginMap[item]; ok {
			finalPs = append(finalPs, p)
		}
	}
	return finalPs, nil
}

func ScanLocalImage(ctx context.Context, imageName string,
	enabledPlugins []string, pluginParams []string,
	reportService *report.ReportService) error {
	veinmindRuntime, err := docker.New()
	if err != nil {
		return err
	}
	imageIDs, err := veinmindRuntime.FindImageIDs(imageName)
	if err != nil {
		return err
	}
	finalPs, err := FindTargetPlugins(ctx, enabledPlugins)
	if err != nil {
		return err
	}
	for _, id := range imageIDs {
		image, err := veinmindRuntime.OpenImageByID(id)
		if err != nil {
			log.Error(err)
			continue
		}
		err = ScanImage(ctx, finalPs, image, reportService,
			specflags.WithSpecFlags(pluginParams))
		if err != nil {
			log.Error(err)
		}
	}
	return nil
}

func ScanImage(ctx context.Context, rang plugin.ExecRange, image api.Image,
	reportService *report.ReportService, opts ...plugin.ExecOption) error {
	opts = append(opts, plugin.WithExecInterceptor(func(
		ctx context.Context, plug *plugin.Plugin, c *plugin.Command,
		next func(context.Context, ...plugin.ExecOption) error,
	) error {
		// Register Service
		reg := service.NewRegistry()
		reg.AddServices(log.WithFields(log.Fields{
			"plugin":  plug.Name,
			"command": path.Join(c.Path...),
		}))
		reg.AddServices(reportService)

		// Next Plugin
		return next(ctx, reg.Bind())
	}))
	return cmd.ScanImage(ctx, rang, image, opts...)
}
