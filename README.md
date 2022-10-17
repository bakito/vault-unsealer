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
    vault-unsealer.bison-group.com/stateful-set-disable: vault
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
    vault-unsealer.bison-group.com/stateful-set: vault
  name: unseal-pw
type: Opaque
stringData:
  username: <username>
  password: <password>
  secretPath: /path/to/unsealKey/secret

```
