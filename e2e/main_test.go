package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envfuncs"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testEnv         env.Environment
	kindClusterName string
	argocdNamespace = "argocd"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("Skipping e2e tests")
		os.Exit(0)
	}
	config, err := envconf.NewFromFlags()

	if err != nil {
		fmt.Println("Could not create config from env", err)
	}

	testEnv = env.NewWithConfig(config)
	kindClusterName = envconf.RandomName("e2e-local-argocd-renderer", 20)

	testEnv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
		envfuncs.CreateNamespace(argocdNamespace),
	)

	testEnv.Finish(
		envfuncs.DeleteNamespace(argocdNamespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit(testEnv.Run(m))
}
