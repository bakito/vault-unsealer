# vault-unsealer

A kubernetes controller that can auto unseal vault pods.

## Secrets

### With Keys in Secret

```yaml
apiVersion: v1
data:
  unsealKey1: <...>
  unsealKey2: <...>
  unsealKey3: <...>
  unsealKey4: <...>
  unsealKey5: <...>
kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
  name: unseal
type: Opaque
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
