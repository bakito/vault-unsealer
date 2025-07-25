# vault-unsealer

A kubernetes controller that can auto-unseal vault/openbao pods.

## Labels / Annotations

### StatefulSet

Use the label `vault-unsealer.bakito.net/stateful-set` to unseal a kubernetes vault StatefulSet in the same namespace

Example: 

```yaml
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
```

### External

Use the label `vault-unsealer.bakito.net/external` flag a secret for usage to handle external vaults. The value specifies the seal check interval duration

Use the annotation `vault-unsealer.bakito.net/external-source` to define a vault where the unseal keys are stored

Use the annotation `vault-unsealer.bakito.net/external-targets` to define the vaults to be unsealed. The value is semicolon separated

```yaml
  labels:
    vault-unsealer.bakito.net/external: '5m'
  annotations:
    vault-unsealer.bakito.net/external-source: https://vault.bakito.org:8200
    vault-unsealer.bakito.net/external-targets: https://vault-1.bakito.org:8200;https://vault-2.bakito.org:8200
```

## Secrets

Secrets must use one of the labels or annotations described above.

### With Keys in Secret

Unseal keys can directly be stored in a secret.
The keys must have the prefix `unsealKey`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
  name: vault-unsealer-config
type: Opaque
data:
  unsealKey1: <...>
  unsealKey2: <...>
  unsealKey3: <...>
  unsealKey4: <...>
  unsealKey5: <...>
```

### With Vault userpass

If the unseal keys are stored in vault itself, [`userpass`](https://developer.hashicorp.com/vault/docs/auth/userpass)
access can be configured.

| Key        | Description                                                                                                      |
|------------|------------------------------------------------------------------------------------------------------------------|
| username   | The username for vault userpass access.                                                                          |
| password   | The password for vault userpass access.                                                                          |
| mountPath  | The vault mount path. Optional.                                                                                  |
| secretPath | The secret path within vault . <br/>Do NOT add the /data path element as it is required by the vault cli or API. |

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
  name: vault-unsealer-config-userpass
type: Opaque
data:
  username: <...>
  password: <...>
  mountPath:  <...>
  secretPath: <...>
```

#### Test

```bash
# Get Token
vault login -method=userpass username=<username>

# Read the secret (for kv version 2 '/data' must be added to the secret path,
# but only for the cli, not the unsealer secret)
vault read kv/data/unsealer
```

### With Vault kubernetes service account

Another option to access unseal keys stored in vault is to
use [`kubernetes`](https://developer.hashicorp.com/vault/docs/auth/kubernetes) auth, where the service account of the
unsealer must be granted access to vault.

| Key        | Description                                                                                                      |
|------------|------------------------------------------------------------------------------------------------------------------|
| role       | The role the kubernetes service account is assigned to.                                                          |
| mountPath  | The vault mount path. Optional.                                                                                  |
| secretPath | The secret path within vault . <br/>Do NOT add the /data path element as it is required by the vault cli or API. |

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    vault-unsealer.bakito.net/stateful-set: vault
  name: vault-unsealer-config-kubernetes
type: Opaque
data:
  role: <...>
  mountPath:  <...>
  secretPath: <...>
```

#### Test

```bash
# Get Token
vault write auth/kubernetes/login role=<role> jwt=<k8s-token>

# Login wit received vault token
vault login

# Read the secret (for kv version 2 '/data' must be added to the secret path,
# but only for the cli, not the unsealer secret)
vault read kv/data/unsealer
```

### Required vault policy for userpass and kubernetes auth

```hcl
# allow access to read the secret
path "path/to/your/unseal/secret" {
  capabilities = ["read"]
}
# allow access to read the mounts (used to check the kv version of the secret engine)
path "sys/mounts" {
  capabilities = ["read"]
}
```
