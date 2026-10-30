# Archestra Crossplane provider

Manage Archestra agents, MCP servers, teams, projects, credentials, and other
resources as Kubernetes objects.

## Install

Install [Crossplane](https://docs.crossplane.io/latest/get-started/install/) first.
The provider is tested with Crossplane 2.2.1. Choose a released version from
[Upbound Marketplace](https://marketplace.upbound.io/providers/archestra/provider-archestra)
and pin it in your manifest; this example uses v3.0.3:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-archestra
spec:
  package: xpkg.upbound.io/archestra/provider-archestra:v3.0.3
```

Save this as `provider.yaml`, then wait for the provider to become healthy:

```bash
kubectl apply -f provider.yaml
kubectl wait provider.pkg.crossplane.io/provider-archestra --for=condition=Healthy --timeout=5m
```

## Configure credentials

Create an Archestra API key under **Settings → API Keys**. Set `base_url` to your
Archestra API endpoint reachable from the provider pod. The default,
`http://localhost:9000`, points inside the pod and usually needs overriding.

```bash
kubectl create secret generic archestra-creds -n crossplane-system \
  --from-literal=credentials='{"api_key":"arch_REPLACE_ME","base_url":"https://archestra.example.com"}'
```

The following namespaced `ProviderConfig` uses that Secret. Save it as
`providerconfig.yaml` and apply it with `kubectl apply -f providerconfig.yaml`:

```yaml
apiVersion: archestra.m.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
  namespace: crossplane-system
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: archestra-creds
      key: credentials
```

## Create a team

Save this as `team.yaml` and run `kubectl apply -f team.yaml`:

```yaml
apiVersion: team.archestra.m.crossplane.io/v1alpha1
kind: Team
metadata:
  name: engineering
  namespace: crossplane-system
spec:
  forProvider:
    name: Engineering
    description: Engineering team
  providerConfigRef:
    kind: ProviderConfig
    name: default
```

```bash
kubectl wait teams.team.archestra.m.crossplane.io/engineering -n crossplane-system --for=condition=Ready --timeout=5m
kubectl get teams.team.archestra.m.crossplane.io/engineering -n crossplane-system -o yaml
```

Inputs belong in `spec.forProvider`; observed values appear in
`status.atProvider`. Use `*Ref` or `*Selector` fields where available to refer to
other managed resources. Sensitive inputs use Kubernetes Secret references. See the [relationship reference guide](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/crossplane/RELATIONSHIPS.md) for Team hierarchy/role examples and the coverage audit.

Deleting this managed resource deletes its Archestra object by default. To retain
the object, set `spec.deletionPolicy: Orphan` before deleting the manifest.

## API reference and examples

The **Managed Resources** tab on
[Upbound Marketplace](https://marketplace.upbound.io/providers/archestra/provider-archestra)
contains the field reference for each package version. Both API scopes are included:

| Scope | API group suffix | Credentials configuration |
| --- | --- | --- |
| Namespaced | `archestra.m.crossplane.io` | `ProviderConfig` in the resource namespace, or `ClusterProviderConfig` |
| Cluster | `archestra.crossplane.io` | Cluster-scoped `ProviderConfig` |

Managed resources use `v1alpha1`; credential configuration uses `v1beta1`.
For cluster-scoped resources, omit `metadata.namespace` and the
`providerConfigRef.kind` field. See the
[credential examples](https://github.com/archestra-ai/terraform-provider-archestra/tree/main/crossplane/examples).

Use the resource examples as templates: replace placeholders and expressions
such as `${...}`, supply referenced resources and Secrets, and select your
`providerConfigRef` before applying them.

For upgrades from v2, read the
[v3 migration guide](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/MIGRATION-v3.md).
Report bugs or request features in
[GitHub issues](https://github.com/archestra-ai/terraform-provider-archestra/issues).

## Contributing

See the [development guide](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/crossplane/CONTRIBUTING.md)
for building, testing, and adding resources.

## License

[Apache 2.0](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/LICENSE).
