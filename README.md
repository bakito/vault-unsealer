# vault-unsealer

A kubernetes controller that can auto unseal vault pods.

## Secrets

### With Keys in Secret

```yaml
apiVersion: v1
data:
  unsealKey1: RlZkZTNyRzZJKzBOOXZIbkRTMUdyMnF1c0UxTXAxOE9HWUc1VVlFbmptaHc=
  unsealKey2: cXhyeGE0c08xemNTenBrWHh4cUFRanpSLy96a0ZUQjFMYnl2SC9adldiZGM=
  unsealKey3: bXNiS2V2U0lQNms4NkI3NlRPdnVmL0xZdmR2RGJmcEZkWWlZSVVpa3pJSkw=
  unsealKey4: ejRPUmZHZE0zVHVWQzVVTk5oVlNrWC85OWtoMGRzQjduZytzN1ViLytyT0Q=
  unsealKey5: ZlR4eGtKdW12YllwczJZL0gvVElhQURhZnRRZXZ5NG1qdXBLMFJNYlpVVHo=
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
