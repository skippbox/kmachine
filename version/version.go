package version

var (
	// Version should be updated by hand at each release
        // We use the same version number as Kubernetes releases
	Version = "1.2.0"

	// GitCommit will be overwritten automatically by the build system
	GitCommit = "HEAD"
)
