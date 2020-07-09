/*
Copyright 2020 Michael Bridgen <mikeb@squaremobius.net>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	imagev1alpha1 "github.com/squaremo/image-update/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// for Eventually
const (
	timeout  = time.Second * 30
	interval = time.Second * 1
	// indexInterval = time.Second * 1
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sMgr ctrl.Manager
var imageRepoReconciler *ImageRepositoryReconciler
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = imagev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sMgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	imageRepoReconciler = &ImageRepositoryReconciler{
		Client: k8sMgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ImageRepository"),
		Scheme: scheme.Scheme,
	}
	err = (imageRepoReconciler).SetupWithManager(k8sMgr)
	Expect(err).ToNot(HaveOccurred())

	// this must be started for the caches to be running, and thereby
	// for the client to be usable.
	go func() {
		err = k8sMgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sMgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = Describe("ImageRepository controller", func() {
	It("expands the canonical image name", func() {
		repo := imagev1alpha1.ImageRepository{
			Spec: imagev1alpha1.ImageRepositorySpec{
				Image: "alpine",
			},
		}
		imageRepoName := types.NamespacedName{
			Name:      "alpine-image",
			Namespace: "default",
		}

		repo.Name = imageRepoName.Name
		repo.Namespace = imageRepoName.Namespace

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		r := imageRepoReconciler
		err := r.Create(ctx, &repo)
		Expect(err).ToNot(HaveOccurred())

		var repoAfter imagev1alpha1.ImageRepository
		Eventually(func() bool {
			err = r.Get(context.Background(), imageRepoName, &repoAfter)
			return err == nil && repoAfter.Status.CanonicalImageName != ""
		}, timeout, interval).Should(BeTrue())
		Expect(repoAfter.Name).To(Equal("alpine-image"))
		Expect(repoAfter.Namespace).To(Equal("default"))
		Expect(repoAfter.Status.CanonicalImageName).To(Equal("index.docker.io/library/alpine"))
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})