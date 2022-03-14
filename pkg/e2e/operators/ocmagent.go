package operators

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/osde2e/pkg/common/alert"
	"github.com/openshift/osde2e/pkg/common/helper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/pkg/apis/ocmagent/v1alpha1"
)

var (
	ocmAgentTestPrefix               = "[Suite: informing] [OSD] OCM Agent Operator"
	ocmAgentBasicTest                = ocmAgentTestPrefix + " Basic Test"
	ocmAgentDeploymentReplicas int32 = 1
	ocmAgentTokenRefName             = "ocm-access-token"
	ocmAgentCofigRefName             = "ocm-agent-config"
	ocmAgentImageName                = "quay.io/app-sre/ocm-agent:latest"
	ocmURL                           = "https://api.stage.openshift.com"
)

func init() {
	alert.RegisterGinkgoAlert(ocmAgentBasicTest, "SD_SREP", "@ocm-agent-operator", "sd-cicd-alerts", "sd-cicd@redhat.com", 4)
}

var _ = ginkgo.Describe(ocmAgentBasicTest, func() {
	var (
		operatorNamespace    = "openshift-ocm-agent-operator"
		operatorName         = "ocm-agent-operator"
		ocmAgentResourceName = "ocm-agent"

		clusterRoles = []string{
			"ocm-agent-operator",
		}
		clusterRoleBindings = []string{
			"ocm-agent-operator",
		}
		// servicePort = 8081
	)
	h := helper.New()
	checkClusterServiceVersion(h, operatorNamespace, operatorName)
	checkDeployment(h, operatorNamespace, operatorName, 1)
	checkClusterRoles(h, clusterRoles, true)
	checkClusterRoleBindings(h, clusterRoleBindings, true)
	// checkService(h, operatorNamespace, operatorName, servicePort)
	// checkUpgrade(helper.New(), operatorNamespace, operatorName, operatorName, "ocm-agent-operator-registry")

	ginkgo.Context("When ocm-agent resource exist or created", func() {
		existingOcmAgent, _ := getOcmAgent(ocmAgentResourceName, operatorNamespace, h)
		if existingOcmAgent != nil {
			// check Deployment status for existing OcmAgent resource
			checkDeployment(h, operatorNamespace, operatorName, ocmAgentDeploymentReplicas)
		} else {
			// create OcmAgent resource if it's not exist
			ocmAgent := makeOcmAgent(ocmAgentResourceName, operatorNamespace)
			err := createOcmAgent(ocmAgent, operatorNamespace, h)
			Expect(err).NotTo(HaveOccurred())

			// Wait a minute and see if ocm-agent deployment has created
			err = wait.Poll(1*time.Minute, 2*time.Minute, func() (bool, error) {
				checkDeployment(h, operatorNamespace, ocmAgentResourceName, ocmAgentDeploymentReplicas)
				if err != nil {
					return false, fmt.Errorf("ocm-agent Deployment not created successfully")
				}
			})
		}
	})
})

func makeOcmAgent(name string, ns string) ocmagentv1alpha1.OcmAgent {
	ocmAgent := ocmagentv1alpha1.OcmAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OcmAgent",
			APIVersion: "ocmagent.managed.openshift.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: ocmagentv1alpha1.OcmAgentSpec{
			AgentConfig: ocmagentv1alpha1.AgentConfig{
				OcmBaseUrl: ocmURL,
				Services:   []string{"service_logs"},
			},
			OcmAgentConfig: ocmAgentCofigRefName,
			OcmAgentImage:  ocmAgentImageName,
			TokenSecret:    ocmAgentTokenRefName,
			Replicas:       ocmAgentDeploymentReplicas,
		},
	}
	return ocmAgent
}

func createOcmAgent(OcmAgent ocmagentv1alpha1.OcmAgent, operatorNamespace string, h *helper.H) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(OcmAgent.DeepCopy())
	if err != nil {
		return err
	}
	unstructuredObj := unstructured.Unstructured{Object: obj}
	_, err = h.Dynamic().Resource(schema.GroupVersionResource{
		Group: "ocmagent.managed.openshift.io", Version: "v1alpha1", Resource: "OcmAgent",
	}).Namespace(operatorNamespace).Create(context.TODO(), &unstructuredObj, metav1.CreateOptions{})
	return err
}

func deleteOcmAgent(name string, operatorNamespace string, h *helper.H) error {
	return h.Dynamic().Resource(schema.GroupVersionResource{
		Group: "ocmagent.managed.openshift.io", Version: "v1alpha1", Resource: "OcmAgent",
	}).Namespace(operatorNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func getOcmAgent(name string, ns string, h *helper.H) (*ocmagentv1alpha1.OcmAgent, error) {
	ucObj, err := h.Dynamic().Resource(schema.GroupVersionResource{
		Group: "ocmagent.managed.openshift.io", Version: "v1alpha1", Resource: "OcmAgent",
	}).Namespace(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error retrieving OcmAgent: %v", err)
	}
	var ocmagent ocmagentv1alpha1.OcmAgent
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(ucObj.UnstructuredContent(), &ocmagent)
	if err != nil {
		// This, however, is probably error-worthy because it means our OcmAgent
		// has been messed with or something odd's occurred
		return nil, fmt.Errorf("error parsing ocmagent into object")
	}

	return &ocmagent, nil
}
