# vault-unsealer

A kubernetes controller that can auto unseal vault pods.

## Secrets

### With Keys in Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
  name: unseal
type: Opaque
data:
  unsealKey1: <...>
  unsealKey2: <...>
  unsealKey3: <...>
  unsealKey4: <...>
  unsealKey5: <...>
```

### With Vault userpass

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
  name: unseal-pw
type: Opaque
stringData:
  username: <username>
  password: <password>
  # do not add `/data` to the path
  secretPath: /path/to/unsealKey/secret
```

#### Required Policy

```hcl
path "path/to/your/unseal/secret" {
  capabilities = ["read"]
}
path "sys/mounts" {
  capabilities = ["read"]
}
```

### With Vault kubernetes service account

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
  name: unseal-pw
type: Opaque
stringData:
  role: <role>
  # do not add `/data` to the path
  secretPath: /path/to/unsealKey/secret
```
