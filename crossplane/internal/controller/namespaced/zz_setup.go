// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/v2/pkg/controller"

	agent "github.com/archestra-ai/provider-archestra/internal/controller/namespaced/agent/agent"
	toolbatch "github.com/archestra-ai/provider-archestra/internal/controller/namespaced/agent/toolbatch"
	registrycatalogitem "github.com/archestra-ai/provider-archestra/internal/controller/namespaced/mcp/registrycatalogitem"
	serverinstallation "github.com/archestra-ai/provider-archestra/internal/controller/namespaced/mcp/serverinstallation"
	toolinvocationpolicydefault "github.com/archestra-ai/provider-archestra/internal/controller/namespaced/policy/toolinvocationpolicydefault"
	providerconfig "github.com/archestra-ai/provider-archestra/internal/controller/namespaced/providerconfig"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		agent.Setup,
		toolbatch.Setup,
		registrycatalogitem.Setup,
		serverinstallation.Setup,
		toolinvocationpolicydefault.Setup,
		providerconfig.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated creates all controllers with the supplied logger and adds them to
// the supplied manager gated.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		agent.SetupGated,
		toolbatch.SetupGated,
		registrycatalogitem.SetupGated,
		serverinstallation.SetupGated,
		toolinvocationpolicydefault.SetupGated,
		providerconfig.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
