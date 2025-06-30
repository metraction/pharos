// Converts prometheus metrics to a PharosScanTask struct

package prometheus

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"time"

	hwmodel "github.com/metraction/handwheel/model"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type DockerConfigJSON struct {
	Auths map[string]struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Auth     string `json:"auth"` // Base64 encoded username:password
	} `json:"auths"`
	// +optional
	HTTPHeaders map[string]string `json:"HttpHeaders,omitempty"`
}

type PharosScanTaskCreator struct {
	Logger            *zerolog.Logger
	Config            *model.Config
	DockerConfigJSONs []DockerConfigJSON // Use the Kubernetes Secret type for image pull secrets
}

func NewPharosScanTaskCreator(config *model.Config) *PharosScanTaskCreator {
	return &PharosScanTaskCreator{
		Logger:            logging.NewLogger("info"),
		Config:            config,
		DockerConfigJSONs: []DockerConfigJSON{},
	}
}

func (pst *PharosScanTaskCreator) WithImagePullSecrets() *PharosScanTaskCreator {

	// Try in-cluster config first, fallback to default kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
		pst.Logger.Warn().Err(err).Msg("Failed to create Kubernetes client")
		if home := homedir.HomeDir(); home != "" {
			kubeconfig := filepath.Join(home, ".kube", "config")
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	namespace := pst.Config.Prometheus.Namespace // Default namespace for Pharos, can be overridden by
	pst.Logger.Info().Str("namespace", namespace).Msg("Using Kubernetes client to fetch image pull secrets")
	secrets, err := clientset.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: "type=kubernetes.io/dockerconfigjson",
	})
	if err != nil {
		pst.Logger.Error().Err(err).Msgf("Failed to list secrets in namespace %s", namespace)
		return pst
	}
	for _, secret := range secrets.Items {
		pst.Logger.Info().Str("secret", secret.Name).Msg("Found image pull secret")
		if secret.Data != nil {
			if dockerConfig, exists := secret.Data[".dockerconfigjson"]; exists {
				var dockerConfigJSON DockerConfigJSON
				err := json.Unmarshal(dockerConfig, &dockerConfigJSON)
				if err != nil {
					pst.Logger.Error().Err(err).Msg("Failed to unmarshal docker config JSON")
				}
				pst.DockerConfigJSONs = append(pst.DockerConfigJSONs, dockerConfigJSON)
			} else {
				pst.Logger.Warn().Str("secret", secret.Name).Msg("Secret does not contain dockerconfigjson key")
			}
		}
	}
	return pst
}

func (pst *PharosScanTaskCreator) Result(metric hwmodel.ImageMetric) []model.PharosScanTask2 {
	// Look for a matching DockerConfigJSON for the image

	repo := "docker.io"
	matches := regexp.MustCompile(`^([^/]+)/`).FindStringSubmatch(metric.Image_spec)
	pharosRepoAuth := model.PharosRepoAuth{}
	if len(matches) > 1 {
		repo = matches[1]
	}
	for _, dockerConfig := range pst.DockerConfigJSONs {
		if auth, exists := dockerConfig.Auths[repo]; exists {
			pst.Logger.Debug().Str("image", metric.Image_spec).Msg("Found matching Docker configJSON for image")
			pharosRepoAuth.Authority = repo
			pharosRepoAuth.Username = auth.Username
			pharosRepoAuth.Password = auth.Password
			continue
		}
	}

	now := time.Now()
	pharosScanTask := model.PharosScanTask2{
		ImageSpec: metric.Image_spec,
		Platform:  pst.Config.Prometheus.Platform, // Default platform, can be adjusted as needed
		AuthDsn:   pharosRepoAuth.ToDsn(),
		Created:   now,
		Updated:   now,
		Timeout:   time.Second * 180, // 3 minutes
	}
	return []model.PharosScanTask2{pharosScanTask}
}
