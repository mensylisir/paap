package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type ContainerImageCopyOptions struct {
	TargetInsecure bool
	TargetUsername string
	TargetPassword string
}

func CopyContainerImageIfMissing(ctx context.Context, source, target string, opts ContainerImageCopyOptions) error {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if source == "" || target == "" {
		return fmt.Errorf("source and target image are required")
	}

	srcRef, err := name.ParseReference(source)
	if err != nil {
		return fmt.Errorf("parse source image %q: %w", source, err)
	}
	dstRef, targetRemoteOptions, err := containerImageTargetRef(ctx, target, opts)
	if err != nil {
		return err
	}

	if _, err := remote.Head(dstRef, targetRemoteOptions...); err == nil {
		return nil
	}

	image, err := remote.Image(srcRef, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("pull source image %q: %w", source, err)
	}
	if err := remote.Write(dstRef, image, targetRemoteOptions...); err != nil {
		return fmt.Errorf("push target image %q: %w", target, err)
	}
	return nil
}

func ContainerImageExists(ctx context.Context, target string, opts ContainerImageCopyOptions) (bool, error) {
	dstRef, targetRemoteOptions, err := containerImageTargetRef(ctx, target, opts)
	if err != nil {
		return false, err
	}
	if _, err := remote.Head(dstRef, targetRemoteOptions...); err != nil {
		return false, err
	}
	return true, nil
}

func containerImageTargetRef(ctx context.Context, target string, opts ContainerImageCopyOptions) (name.Reference, []remote.Option, error) {
	targetNameOptions := []name.Option{}
	if opts.TargetInsecure {
		targetNameOptions = append(targetNameOptions, name.Insecure)
	}
	dstRef, err := name.ParseReference(target, targetNameOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("parse target image %q: %w", target, err)
	}
	targetRemoteOptions := []remote.Option{remote.WithContext(ctx)}
	if strings.TrimSpace(opts.TargetUsername) != "" || strings.TrimSpace(opts.TargetPassword) != "" {
		targetRemoteOptions = append(targetRemoteOptions, remote.WithAuth(&authn.Basic{
			Username: opts.TargetUsername,
			Password: opts.TargetPassword,
		}))
	}
	return dstRef, targetRemoteOptions, nil
}
