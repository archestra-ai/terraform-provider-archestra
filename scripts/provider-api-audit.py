#!/usr/bin/env python3
"""Compare provider request builders with an upstream OpenAPI snapshot.

Static evidence only: a mapped/generated resource is not a tested controller.
No upstream strings are executed; inputs are local JSON and Go source files.
"""
import argparse
import json
from pathlib import Path
import re


def canonical_path(path):
    return re.sub(r"\{[^}]+\}|%s", "{}", path)


def request_builders(source):
    result = {}
    for match in re.finditer(
        r"func New(\w+?)Request(?:WithBody)?\([^\n]+\{(.*?)(?=\nfunc |\Z)",
        source, re.S,
    ):
        name, body = match.groups()
        path = re.search(r'operationPath := fmt.Sprintf\("([^"]+)"', body)
        method = re.search(r'http.NewRequest\("([^"]+)"', body)
        if path and method:
            result[name] = (method[1], canonical_path(path[1]))
    return result


def schema_fields(block, prefix=""):
    result = []
    for name, value in sorted(block.get("attributes", {}).items()):
        path = prefix + name
        result.append({"path": path, **{key: value.get(key, False) for key in ["required", "optional", "computed", "sensitive", "write_only"]}})
        nested = value.get("nested_type")
        if nested:
            result.extend(schema_fields(nested, path + "."))
    for name, value in sorted(block.get("block_types", {}).items()):
        result.extend(schema_fields(value["block"], prefix + name + "."))
    return result


def audit(root, spec, terraform_schema=None, wire_contracts=None):
    operations = {
        (method.upper(), canonical_path(path)): operation
        for path, methods in spec["paths"].items()
        for method, operation in methods.items()
        if method in {"get", "post", "put", "patch", "delete", "head", "options"}
    }
    builders = {}
    for file in sorted((root / "internal/client").glob("*.go")):
        if not file.name.endswith("_test.go"):
            builders.update(request_builders(file.read_text()))
    mappings = set(re.findall(
        r'"(archestra_[a-z_]+)"',
        (root / "crossplane/config/external_name.go").read_text(),
    ))
    resources = []
    referenced = set()
    declared = {}
    for file in sorted((root / "internal/provider").glob("resource_*.go")):
        if file.name.endswith("_test.go"):
            continue
        source = file.read_text()
        name = re.search(r'TypeName\s*=\s*req.ProviderTypeName\s*\+\s*"(_[a-z_]+)"', source)
        if name:
            declared["archestra" + name[1]] = file
    if wire_contracts is not None:
        for name, contract in wire_contracts.items():
            if contract.get("resource_source"):
                declared[name] = root / contract["resource_source"]
            elif name not in declared:
                raise ValueError("Registered resource has no auditable source: " + name)
        declared = {name: declared[name] for name in wire_contracts}
    for name, file in sorted(declared.items()):
        source = file.read_text()
        calls = sorted(set(re.findall(r'r.client\.(\w+?)(?:WithBody)?WithResponse\(', source)))
        contract = (wire_contracts or {}).get(name)
        if contract is not None:
            calls = contract.get('api_methods', calls)
        runtime_schema = (terraform_schema or {}).get("provider_schemas", {}).get("registry.terraform.io/archestra-ai/archestra", {}).get("resource_schemas", {}).get(name)
        runtime_fields = runtime_schema.get("block", {}).get("attributes", {}) if runtime_schema else None
        endpoints = []
        for call in calls:
            endpoint = builders.get(call)
            if endpoint:
                referenced.add(endpoint)
            operation = operations.get(endpoint, {})
            request = operation.get("requestBody", {}).get("content", {}).get("application/json", {}).get("schema", {})
            properties = request.get("properties", {})
            endpoints.append({
                "request_fields": sorted(properties),
                "declared_request_field_mappings": {field: contract['wire_fields'][field] for field in sorted(properties) if contract and field in contract['wire_fields']} if contract else None,
                "request_fields_without_declared_mapping": sorted(field for field, shape in properties.items() if shape.get('not') != {} and field not in contract['wire_fields']) if contract else None,
                "api_forbidden_request_fields": sorted(key for key, value in properties.items() if value.get("not") == {}),
                "retired_request_fields": sorted(key for key, value in properties.items() if value.get("not") == {} and (runtime_fields is None or any(v.get("optional") or v.get("required") for n,v in runtime_fields.items() if n == re.sub(r"(?<!^)(?=[A-Z])", "_", key).lower()))),
                "query_parameters": sorted(parameter["name"] for parameter in operation.get("parameters", []) if parameter.get("in") == "query"),
                "builder": call,
                "method": endpoint[0] if endpoint else None,
                "path": endpoint[1] if endpoint else None,
                "present_in_upstream": endpoint in operations if endpoint else None,
            })
        tests = file.with_name(file.stem + "_test.go")
        test_source = tests.read_text() if tests.exists() else ""
        runtime_schema = (terraform_schema or {}).get("provider_schemas", {}).get("registry.terraform.io/archestra-ai/archestra", {}).get("resource_schemas", {}).get(name)
        resources.append({
            "schema_version": runtime_schema["version"] if runtime_schema else None,
            "fields": schema_fields(runtime_schema["block"]) if runtime_schema else None,
            "terraform_resource": name,
            "source": str(file.relative_to(root)),
            "crossplane_configured": name in mappings,
            "endpoints": endpoints,
            "import_implemented": contract.get("import_implemented", " ImportState(" in source) if contract else " ImportState(" in source,
            "acceptance_tests_declared": len(re.findall(r"func TestAcc", test_source)),
            "acceptance_execution": "unverified",
            "crossplane_reconciliation": "unverified",
            "note": "Method-receiver API inventory including helpers; derived/synthetic request mappings still require review." if contract else "Direct call inventory; shared helpers, fields, permissions and lifecycle need review.",
        })
    return {
        "format_version": 1,
        "evidence_level": "static inventory; no runtime coverage implied",
        "terraform_count": len(resources),
        "crossplane_configured_count": sum(r["crossplane_configured"] for r in resources),
        "resources": resources,
        "unreferenced_mutating_operations": [
            {"method": method, "path": path, "operation_id": op.get("operationId")}
            for (method, path), op in sorted(operations.items())
            if method in {"POST", "PUT", "PATCH", "DELETE"} and (method, path) not in referenced
        ],
    }


def markdown(report):
    lines = ["# Provider API inventory", "", report["evidence_level"], "",
             f"Terraform: {report['terraform_count']}; Crossplane configured: {report['crossplane_configured_count']}.", "",
             "Mapping counts are configuration coverage, not successful CRUD or reconciliation coverage.", "",
             "| Terraform resource | Crossplane configured | Import | Missing direct API calls / retired request fields | Request fields needing review | Acceptance tests declared |",
             "|---|---|---|---|---|---|"]
    for r in report["resources"]:
        missing = [f"{e['method']} {e['path']}" for e in r["endpoints"] if e["present_in_upstream"] is False]
        missing += [f"{e['builder']}: retired {', '.join(e.get('retired_request_fields', []))}" for e in r["endpoints"] if e.get("retired_request_fields")]
        requests = [f"{e['builder']}: {', '.join(e['request_fields_without_declared_mapping'])}" for e in r['endpoints'] if e.get('request_fields_without_declared_mapping')]
        reviewed = '; '.join(requests) or ('none detected' if all(e.get('request_fields_without_declared_mapping') is not None for e in r['endpoints']) else 'wire contracts not supplied')
        lines.append(f"| `{r['terraform_resource']}` | {r['crossplane_configured']} | {r['import_implemented']} | {'; '.join(missing) or 'none detected'} | {reviewed} | {r['acceptance_tests_declared']} |")
    lines += ["", "Unreferenced mutations are review candidates, not an automatically inferred resource denominator.",
              "RPCs, inference, OAuth flows, logs and transient executions must not become managed resources just to increase a percentage.",
              "Request fields without a declared mapping require review: derived wire values and backend-owned fields need explicit justification; a clean GET-field gate alone does not prove write coverage."]
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--terraform-schema", type=Path)
    parser.add_argument("--wire-contracts", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--markdown", type=Path)
    args = parser.parse_args()
    report = audit(args.root, json.loads(args.spec.read_text()),
                   json.loads(args.terraform_schema.read_text()) if args.terraform_schema else None,
                   json.loads(args.wire_contracts.read_text()) if args.wire_contracts else None)
    args.output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
    if args.markdown:
        args.markdown.write_text(markdown(report))


if __name__ == "__main__":
    main()
