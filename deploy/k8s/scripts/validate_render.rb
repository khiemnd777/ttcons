# frozen_string_literal: true

require "yaml"

def assert(condition, message)
  raise message unless condition
end

def documents(path)
  YAML.load_stream(File.read(path)).compact
rescue Psych::SyntaxError => e
  raise "invalid YAML in #{path}: #{e.message}"
end

def named(documents, kind, name)
  documents.find do |document|
    document["kind"] == kind && document.dig("metadata", "name") == name
  end
end

render_path, migration_path, storage_migration_path, hpa_path, ingress_path = ARGV
assert([render_path, migration_path, storage_migration_path, hpa_path, ingress_path].all?,
       "usage: validate_render.rb RENDER MIGRATION STORAGE_MIGRATION HPA INGRESS")

rendered = documents(render_path)
migration_docs = documents(migration_path)
storage_migration_docs = documents(storage_migration_path)
hpa_docs = documents(hpa_path)
ingress_docs = documents(ingress_path)

assert(named(rendered, "Namespace", "consflow"), "base must define the consflow namespace")
assert(named(rendered, "ServiceAccount", "consflow-workload"), "base must define the non-token workload service account")
assert(rendered.none? { |doc| %w[StatefulSet PersistentVolume PersistentVolumeClaim].include?(doc["kind"]) },
       "base must not provision in-cluster stateful data services")
assert(rendered.none? { |doc| doc["kind"] == "Secret" }, "base must not render secret values")
assert(rendered.none? { |doc| doc["kind"] == "HorizontalPodAutoscaler" }, "HPA must remain opt-in")
assert(rendered.none? { |doc| doc["kind"] == "Job" }, "migration Job must remain separate from the application base")

config = named(rendered, "ConfigMap", "consflow-runtime")
assert(config, "base must define consflow-runtime ConfigMap")
secret_like_keys = config.fetch("data", {}).keys.grep(/DATABASE_URL|REDIS_URL|JWT_SECRET|ACCESS_KEY|SECRET_KEY/)
assert(secret_like_keys.empty?, "runtime ConfigMap contains secret-like keys: #{secret_like_keys.join(', ')}")
assert(config.dig("data", "AUTO_MIGRATE") == "false", "backend AUTO_MIGRATE must remain false")
assert(config.dig("data", "RATE_LIMIT_MODE") == "enforce", "production base must enforce rate limits")
assert(config.dig("data", "STORAGE_PROVIDER") == "s3", "multi-replica base must force the S3 provider")
assert(!config.fetch("data", {}).key?("STORAGE_PATH"), "S3-only application base must not configure STORAGE_PATH")
assert(config.dig("data", "STORAGE_S3_BUCKET") == "", "base S3 bucket must fail closed until overlaid")
assert(config.dig("data", "STORAGE_S3_REGION") == "", "base S3 region must fail closed until overlaid")
assert(config.dig("data", "TRUSTED_PROXY_CIDRS")&.start_with?("REPLACE_"),
       "base backend proxy trust must fail closed until overlaid")
assert(config.dig("data", "TRUSTED_PROXY_CIDR")&.start_with?("REPLACE_"),
       "base gateway proxy trust must fail closed until overlaid")
assert(!config.fetch("data", {}).key?("SERVER_NAME"),
       "managed-edge Caddy origin must not request or manage a public hostname/TLS certificate")

%w[admin backend].each do |name|
  deployment = named(rendered, "Deployment", name)
  assert(deployment, "missing #{name} Deployment")
  assert(deployment.dig("spec", "replicas").to_i >= 2, "#{name} must start with at least two replicas")
  assert(deployment.dig("spec", "strategy", "type") == "RollingUpdate", "#{name} must use RollingUpdate")
  assert(deployment.dig("spec", "strategy", "rollingUpdate", "maxUnavailable").to_i == 1,
         "#{name} must replace one replica at a time on the two-node baseline")
  assert(deployment.dig("spec", "strategy", "rollingUpdate", "maxSurge").to_i.zero?,
         "#{name} must not require an unschedulable surge pod under strict host anti-affinity")

  pod_spec = deployment.dig("spec", "template", "spec")
  assert(pod_spec["automountServiceAccountToken"] == false, "#{name} must not mount a service-account token")
  expected_uid = name == "backend" ? 10_001 : 101
  assert(pod_spec.dig("securityContext", "runAsUser") == expected_uid,
         "#{name} must pin the reviewed numeric runtime UID #{expected_uid}")
  assert(pod_spec.dig("securityContext", "runAsGroup") == expected_uid,
         "#{name} must pin the reviewed numeric runtime GID #{expected_uid}")
  required_anti_affinity = pod_spec.dig("affinity", "podAntiAffinity", "requiredDuringSchedulingIgnoredDuringExecution") || []
  assert(required_anti_affinity.any? { |rule| rule["topologyKey"] == "kubernetes.io/hostname" },
         "#{name} must require host anti-affinity")
  spread = pod_spec.fetch("topologySpreadConstraints", [])
  assert(spread.any? { |rule| rule["topologyKey"] == "topology.kubernetes.io/zone" },
         "#{name} must define zone topology spread")

  container = pod_spec.fetch("containers").find { |item| item["name"] == name }
  assert(container, "missing #{name} container")
  assert(container.fetch("image").start_with?("registry.example.invalid/"),
         "#{name} base image must remain an unroutable fail-closed placeholder")
  %w[startupProbe livenessProbe readinessProbe].each do |probe|
    assert(container[probe], "#{name} is missing #{probe}")
  end
  assert(container.dig("resources", "requests", "cpu"), "#{name} is missing CPU request")
  assert(container.dig("resources", "requests", "memory"), "#{name} is missing memory request")
  assert(container.dig("resources", "limits", "cpu"), "#{name} is missing CPU limit")
  assert(container.dig("resources", "limits", "memory"), "#{name} is missing memory limit")
  assert(container.dig("securityContext", "allowPrivilegeEscalation") == false,
         "#{name} must disable privilege escalation")
  assert(container.dig("securityContext", "readOnlyRootFilesystem") == true,
         "#{name} must use a readonly root filesystem")

  service = named(rendered, "Service", name)
  assert(service&.dig("spec", "type") == "ClusterIP", "#{name} Service must remain private ClusterIP")
  pdb = named(rendered, "PodDisruptionBudget", name)
  assert(pdb&.dig("spec", "minAvailable").to_i >= 1, "#{name} must retain at least one available pod")
end

backend = named(rendered, "Deployment", "backend")
backend_container = backend.dig("spec", "template", "spec", "containers").find { |item| item["name"] == "backend" }
backend_mounts = backend_container.fetch("volumeMounts", [])
backend_volumes = backend.dig("spec", "template", "spec").fetch("volumes", [])
assert(backend_mounts.none? { |mount| mount["mountPath"] == "/app/uploads" || mount["mountPath"] == "/storage" },
       "S3-only backend must not mount a local object-storage path")
assert(backend_volumes.none? { |volume| volume["name"] == "uploads" },
       "S3-only backend must not define an upload scratch volume")
backend_env = backend_container.fetch("env").to_h { |item| [item["name"], item["value"]] }
%w[DATABASE_URL_FILE REDIS_URL_FILE REDIS_CACHE_URL_FILE JWT_SECRET_FILE].each do |key|
  assert(backend_env[key]&.start_with?("/var/run/secrets/consflow/"), "backend must source #{key} from a mounted Secret")
end
aws_env = backend_container.fetch("env").select { |item| item["name"]&.start_with?("AWS_") }
%w[AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN].each do |key|
  item = aws_env.find { |candidate| candidate["name"] == key }
  assert(item&.dig("valueFrom", "secretKeyRef", "name") == "consflow-runtime-secrets",
         "#{key} must come from the runtime Secret when static credentials are used")
  assert(item.dig("valueFrom", "secretKeyRef", "optional") == true,
         "#{key} must remain optional so the default credential chain can use workload identity")
end
smtp_password_env = backend_container.fetch("env").find { |item| item["name"] == "SMTP_PASSWORD" }
assert(smtp_password_env&.dig("valueFrom", "secretKeyRef", "name") == "consflow-runtime-secrets",
       "SMTP_PASSWORD must come from the runtime Secret")
assert(smtp_password_env.dig("valueFrom", "secretKeyRef", "optional") == true,
       "SMTP_PASSWORD must remain optional while account recovery is disabled or the relay is unauthenticated")
assert(backend_container.dig("livenessProbe", "httpGet", "path") == "/api/health",
       "backend liveness must use dependency-free /api/health")
assert(backend_container.dig("readinessProbe", "httpGet", "path") == "/api/ready",
       "backend readiness must use DB-aware /api/ready")
backend_secret = backend.dig("spec", "template", "spec", "volumes").find { |volume| volume["name"] == "runtime-secrets" }
backend_secret_items = backend_secret&.dig("secret", "items") || []
assert(backend_secret_items.any? { |item| item["key"] == "database_url" && item["path"] == "database_url" },
       "backend must mount only the runtime database credential at database_url")
assert(backend_secret_items.any? { |item| item["key"] == "redis_cache_url" && item["path"] == "redis_cache_url" },
       "backend must mount the dedicated project explorer cache credential at redis_cache_url")
assert(backend_secret_items.none? { |item| item["key"] == "migration_database_url" },
       "backend must not mount the migration-owner database credential")

network_policies = rendered.select { |document| document["kind"] == "NetworkPolicy" }
assert(network_policies.length == 2, "base must define exactly the reviewed admin/backend ingress policies")
admin_policy = named(rendered, "NetworkPolicy", "admin-ingress")
backend_policy = named(rendered, "NetworkPolicy", "backend-ingress")
assert(admin_policy&.dig("spec", "policyTypes") == ["Ingress"], "admin NetworkPolicy must isolate ingress")
admin_peers = admin_policy.dig("spec", "ingress", 0, "from") || []
assert(admin_peers.any? do |peer|
  peer.dig("namespaceSelector", "matchLabels", "consflow.network/edge") == "enabled" &&
    peer.dig("podSelector", "matchLabels", "consflow.network/edge-client") == "enabled"
end, "admin ingress must require both reviewed edge namespace and pod labels")
assert(admin_policy.dig("spec", "ingress", 0, "ports", 0, "port") == 8080,
       "admin ingress policy must allow only container port 8080")

assert(backend_policy&.dig("spec", "policyTypes") == ["Ingress"], "backend NetworkPolicy must isolate ingress")
backend_ingress = backend_policy.dig("spec", "ingress") || []
admin_peer = backend_ingress.flat_map { |rule| rule["from"] || [] }.find do |peer|
  peer.dig("podSelector", "matchLabels", "app.kubernetes.io/name") == "admin" &&
    peer.dig("podSelector", "matchLabels", "app.kubernetes.io/instance") == "consflow" &&
    !peer.key?("namespaceSelector")
end
monitoring_peer = backend_ingress.flat_map { |rule| rule["from"] || [] }.find do |peer|
  peer.dig("namespaceSelector", "matchLabels", "consflow.network/monitoring") == "enabled" &&
    peer.dig("podSelector", "matchLabels", "consflow.network/metrics-client") == "enabled"
end
assert(admin_peer, "backend ingress must allow only same-namespace admin pods for application traffic")
assert(monitoring_peer, "backend ingress must require both reviewed monitoring namespace and pod labels")
assert(backend_ingress.all? { |rule| rule.dig("ports", 0, "port") == 8080 },
       "backend ingress policy must allow only container port 8080")

admin = named(rendered, "Deployment", "admin")
admin_pod = admin.dig("spec", "template", "spec")
assert(admin_pod.fetch("initContainers", []).empty?,
       "standard Caddy must use cluster DNS directly without a config-render init container")
assert(admin_pod.dig("securityContext", "fsGroup") == 101,
       "admin must grant Caddy UID 101 write access to its ephemeral storage")
admin_container = admin_pod.fetch("containers").find { |item| item["name"] == "admin" }
assert(admin_container&.fetch("command") == ["/usr/bin/caddy"], "admin must run the Caddy binary")
assert(admin_container.fetch("args") == ["run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"],
       "admin must run the reviewed image Caddyfile")
admin_env = admin_container.fetch("env").to_h { |item| [item["name"], item] }
%w[TRUSTED_PROXY_CIDR REAL_IP_HEADER].each do |key|
  assert(admin_env.dig(key, "valueFrom", "configMapKeyRef", "name") == "consflow-runtime",
         "admin must source #{key} from consflow-runtime")
end
admin_mounts = admin_container.fetch("volumeMounts", []).to_h { |mount| [mount["mountPath"], mount["name"]] }
assert(admin_mounts["/config"] == "caddy-config", "admin must provide Caddy's writable config directory")
assert(admin_mounts["/data"] == "caddy-data", "admin must provide Caddy's writable data directory")
admin_volumes = admin_pod.fetch("volumes", []).to_h { |volume| [volume["name"], volume] }
assert(admin_volumes.dig("caddy-config", "emptyDir", "sizeLimit") == "1Mi",
       "admin must bound Caddy's ephemeral config volume")
assert(admin_volumes.dig("caddy-data", "emptyDir", "sizeLimit") == "16Mi",
       "admin must bound Caddy's ephemeral data volume")

job = migration_docs.find { |document| document["kind"] == "Job" }
assert(job, "migration manifest must define a Job")
assert(job.dig("metadata", "name") == "consflow-migrate", "migration base must use the reviewed Job name")
assert(job.dig("spec", "backoffLimit").to_i.zero?, "migration Job must not auto-retry")
assert(job.dig("spec", "template", "spec", "restartPolicy") == "Never", "migration Job restart policy must be Never")
assert(job.dig("spec", "template", "spec", "securityContext", "runAsUser") == 10_001,
       "migration Job must pin backend image UID 10001")
assert(job.dig("spec", "template", "spec", "securityContext", "runAsGroup") == 10_001,
       "migration Job must pin backend image GID 10001")
migrate_container = job.dig("spec", "template", "spec", "containers").find { |item| item["name"] == "migrate" }
assert(migrate_container&.fetch("command") == ["/app/consflow-migrate"], "migration Job must use the dedicated binary")
assert(migrate_container.fetch("image").start_with?("registry.example.invalid/"),
       "migration base image must remain an unroutable fail-closed placeholder")
migrate_env = migrate_container.fetch("env").to_h { |item| [item["name"], item["value"]] }
assert(migrate_env["DATABASE_URL_FILE"] == "/var/run/secrets/consflow/migration_database_url",
       "migration Job must use the mounted migration-owner database secret")
assert(!migrate_env.key?("AUTO_MIGRATE"), "migration Job must not emulate backend startup migration")
migrate_secret = job.dig("spec", "template", "spec", "volumes").find { |volume| volume["name"] == "runtime-secrets" }
migrate_secret_items = migrate_secret&.dig("secret", "items") || []
assert(migrate_secret_items == [{ "key" => "migration_database_url", "path" => "migration_database_url" }],
       "migration Job must mount only the migration-owner database credential")

storage_job = storage_migration_docs.find { |document| document["kind"] == "Job" }
assert(storage_job&.dig("metadata", "name") == "consflow-storage-migrate-dry-run",
       "storage migration base must be a separately named dry-run Job")
assert(storage_job.dig("spec", "template", "spec", "securityContext", "runAsUser") == 10_001,
       "storage migration Job must pin backend image UID 10001")
assert(storage_job.dig("spec", "template", "spec", "securityContext", "runAsGroup") == 10_001,
       "storage migration Job must pin backend image GID 10001")
storage_container = storage_job.dig("spec", "template", "spec", "containers").find do |item|
  item["name"] == "storage-migrate"
end
assert(storage_container&.fetch("command") == ["/app/consflow-storage-migrate"],
       "storage migration Job must use the dedicated operator binary")
assert(!storage_container.key?("args"), "storage migration example must stay dry-run by default")
assert(storage_container.fetch("image").start_with?("registry.example.invalid/"),
       "storage migration base image must remain an unroutable fail-closed placeholder")
storage_env = storage_container.fetch("env").to_h { |item| [item["name"], item["value"]] }
assert(storage_env["DATABASE_URL_FILE"] == "/var/run/secrets/consflow/database_url",
       "storage migration must use the mounted database secret")
assert(storage_env["STORAGE_PROVIDER"] == "s3", "storage migration destination must be S3")
assert(storage_env["STORAGE_PATH"] == "/legacy/uploads",
       "storage migration must read the explicit external legacy STORAGE_PATH")
assert(storage_env["STORAGE_S3_BUCKET"] == "" && storage_env["STORAGE_S3_REGION"] == "",
       "storage migration bucket/region must fail closed until overlaid")
storage_aws_env = storage_container.fetch("env").select { |item| item["name"]&.start_with?("AWS_") }
%w[AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN].each do |key|
  item = storage_aws_env.find { |candidate| candidate["name"] == key }
  assert(item&.dig("valueFrom", "secretKeyRef", "name") == "consflow-runtime-secrets",
         "storage migration #{key} must come from the runtime Secret when used")
  assert(item.dig("valueFrom", "secretKeyRef", "optional") == true,
         "storage migration #{key} must remain optional for workload identity")
end
legacy_claim = storage_job.dig("spec", "template", "spec", "volumes").find { |volume| volume["name"] == "legacy-uploads" }
assert(legacy_claim&.dig("persistentVolumeClaim", "claimName") == "replace-with-legacy-upload-rwx-pvc",
       "storage migration example must require an explicit legacy RWX claim")
assert(legacy_claim.dig("persistentVolumeClaim", "readOnly") == true,
       "storage migration must mount legacy objects read-only")

assert(hpa_docs.count { |document| document["kind"] == "HorizontalPodAutoscaler" } == 2,
       "optional HPA file must define backend and admin autoscalers")
assert(ingress_docs.one? { |document| document["kind"] == "Ingress" },
       "optional ingress example must contain exactly one Ingress")

puts "Kubernetes baseline validation passed (#{rendered.length} base objects)."
