package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func main() {
	log.Info("starting...")
	ctx, cancel := context.WithCancel(context.Background())
	go signalShutdownHandler(cancel)

	warming(ctx)

	log.Info("shutting down...")
}

func warming(ctx context.Context) {
	var wg sync.WaitGroup

	kubeConfig, err := getKubeConfig()
	if err != nil {
		log.WithError(err).Fatal("failed to get kubeconfig")
	}

	for kc, _ := range kubeConfig.Contexts {
		kubeContext := kc

		config, err := getClientConfigWithContext(kubeConfig, kubeContext)
		if err != nil {
			log.WithError(err).Fatal("failed to create client config")
		}

		go func () {
			wg.Add(1)
			err := warmClusterConnection(ctx, kubeContext, config)
			if err != nil {
				log.WithError(err).Warn("fatal error warming cluster")
			}
			wg.Done()
		}()
	}

	wg.Wait()
}

func getClientConfigWithContext(kubeConfig *api.Config, kubeContext string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveClientConfig(
		*kubeConfig,
		kubeContext,
		&clientcmd.ConfigOverrides{},
		rules,
	).ClientConfig()
}

func getKubeConfig() (*api.Config, error) {
	kubeConfigPath, err := findKubeConfig()
	if err != nil {
		return nil, err
	}

	return clientcmd.LoadFromFile(kubeConfigPath)
}

func findKubeConfig() (string, error) {
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		return env, nil
	}
	path, err := homedir.Expand("~/.kube/config")
	if err != nil {
		return "", err
	}
	return path, nil
}

func warmClusterConnection(ctx context.Context, context string, config *rest.Config) error {
	log.Infof("warming %s", context)

	ticker := time.NewTicker(5 * time.Second)

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrapf(err, "failed to create kube client for context: %s", context)
	}

	for  {
		select {
		case <- ticker.C:
			_, err = client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				log.WithError(err).Warnf("failed to contact cluster: %s", context)
			}

		case <-ctx.Done():
			return nil
		}
	}

	return nil
}

func signalShutdownHandler(cancelFunction context.CancelFunc) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	cancelFunction()

	// Safety deadline for exiting
	<-time.After(5 * time.Second)

	log.Info("shutting down...")
	os.Exit(1)
}