# Despliegue en Dokploy (GOWA)

Notas operativas del fork para desplegar GOWA en Dokploy. Todo lo que sigue se
verificó contra un despliegue real de `gowa` (01Oct2026).

> **Este repositorio es público.** No escribas tokens, contraseñas ni `.env` en
> este archivo. Las credenciales viven en `~/.dokploy/` y en el gestor de
> secretos. Los detalles de red de abajo son internos: si prefieres no
> publicarlos, agrega este archivo a `.gitignore` y consérvalo solo en local.

## Acceso al panel

- Panel: `dokploy.hlgcodes.com`. El host `dokploy2.hlgcodes.com` está apagado y
  devuelve 404; si algo apunta ahí, está mal.
- El header de autenticación es `x-api-key` (no `Authorization: Bearer`, y no
  `x-dokploy-token`). Un token válido sobre una ruta inexistente devuelve **404**,
  no 401: ese 404 significa que autenticaste bien.
- SSH al servidor usa el usuario `ubuntu`, nunca `root`
  (`ssh ubuntu@<ip-del-host>`).

### El CLI tiene una trampa con los tokens

`~/.dokploy/config.json` apunta al panel muerto y a un token vencido, así que el
CLI falla si lo dejas leer su propia config. Además el CLI resuelve el token en
este orden:

```
DOKPLOY_API_KEY  ??  DOKPLOY_AUTH_TOKEN        (y exige DOKPLOY_URL presente)
```

Si `DOKPLOY_API_KEY` está definida (aunque esté vencida) **gana** y el token
bueno se ignora. Y el CLI además carga el `.env` del directorio actual en su
propio entorno, así que no lo ejecutes desde `src/`.

Invocación que sí funciona:

```sh
env -u DOKPLOY_API_KEY -u DOKPLOY_API_TOKEN -u DOKPLOY_CLI_API_TOKEN \
  DOKPLOY_URL=https://dokploy.hlgcodes.com \
  DOKPLOY_AUTH_TOKEN=$(cat ~/.dokploy/api-token-hlgcodes-apps) \
  dokploy application one --applicationId <id>
```

El token bueno está en `~/.dokploy/api-token-hlgcodes-apps` (permisos 600). Las
tres variables de entorno mentioned arriba están vencidas en este equipo y hay
que neutralizarlas en **toda** llamada, con `env -u`.

### La API es tRPC: los queries van por GET

Las mutaciones son `POST` a `/api/trpc/<procedimiento>` con cuerpo
`{"json":{...}}`. Los **queries son `GET`** con
`?input={"json":{...}}`; mandarlos por POST da
`Unsupported POST-request to query procedure` (código 405).

```sh
TOKEN=$(cat ~/.dokploy/api-token-hlgcodes-apps)

# query
curl -s -G "https://dokploy.hlgcodes.com/api/trpc/application.one" \
  -H "x-api-key: $TOKEN" \
  --data-urlencode 'input={"json":{"applicationId":"<id>"}}'

# mutación
curl -s -X POST "https://dokploy.hlgcodes.com/api/trpc/application.update" \
  -H "x-api-key: $TOKEN" -H "Content-Type: application/json" \
  -d '{"json":{"applicationId":"<id>","branch":"main"}}'
```

No existe `application.getAll`: para listar apps usa `application.search` con
`{"page":1,"limit":50}` (devuelve `{items:[...]}`). Otros procedimientos útiles:
`domain.byApplicationId`, `domain.create`, `mounts.create`, `deployment.all`,
`github.githubProviders`.

## Trampas del despliegue

### 1. Un Dockerfile en subdirectorio necesita `dockerContextPath`

Este es el error más caro de diagnosticar, porque Dokploy **clona bien** el repo
y aun así falla con errores que apuntan a archivos inexistentes:

```
#14 ERROR: failed to calculate checksum ...: "/docker/entrypoint.sh": not found
#15 ERROR: failed to calculate checksum ...: "/src": not found
```

La causa: Dokploy compone el contexto como
`<code>/<buildPath>/<dirname(dockerfile)>`. Con `dockerfile` =
`./docker/golang.Dockerfile` eso da `code/docker/`, que solo contiene el
Dockerfile y el entrypoint; el `COPY ./src .` no encuentra nada.

En el log del build se reconoce por estas dos líneas:

```
#5 [internal] load build context
#5 transferring context: 2B done      <-- contexto vacío
```

Un contexto sano para este repo pesa ~2.27 MB y `.dockerignore` 225 B.

**Solución:** fijar `dockerContextPath` a `"."` con `buildPath` en `"/"`.

```sh
curl -s -X POST ".../api/trpc/application.update" -H "x-api-key: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"json":{"applicationId":"<id>","buildPath":"/","dockerContextPath":"."}}'
```

`dockerContextPath` es **relativo** y se concatena al directorio base: poner una
ruta absoluta produce `code/etc/dokploy/...` y falla con
`The path ... does not exist`.

### 2. Los límites de CPU están rotos (v0.30.8)

`cpuLimit: "1.0"` se convierte a `1e-09` y el despliegue muere con:

```
invalid cpu value 1e-09: Must be at least 0.001
```

Las cinco apps del host no tienen límites de CPU, así que déjalos en `null`. La
memoria sí funciona y se expresa **en bytes**, no con `"128M"`:

| Campo | Valor | Nota |
|---|---|---|
| `memoryReservation` | `536870912` | 512 MiB |
| `memoryLimit` | `1073741824` | 1 GiB |

### 3. Los montajes son un procedimiento aparte

`mounts` **no** se guarda enviando `mounts` en `application.update`: el update
responde `true` y el campo sigue vacío. Usa `mounts.create` (POST):

```sh
curl -s -X POST ".../api/trpc/mounts.create" -H "x-api-key: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"json":{"type":"bind","hostPath":"/opt/apps/gowa/storages",
       "mountPath":"/app/storages","serviceType":"application",
       "serviceId":"<applicationId>"}}'
```

**Crea los directorios en el host antes del primer deploy.** Dokploy no los
crea: sin ellos el contenedor arranca igual pero **sin persistencia**, y la
sesión de WhatsApp se pierde en cada redespliegue. Esto no se nota hasta que
intentas vincular un dispositivo.

```sh
sudo mkdir -p /opt/apps/gowa/{storages,statics}
sudo chown -R 20001:20000 /opt/apps/gowa   # uid/gid de gowauser
```

Usa **bind mounts**, no named volumes: `docker system prune --volumes` está
prohibido en este host (borraría datos de otros servicios) y los binds son
inmunes a ese comando. El entrypoint hace `chown` de los volúmenes al arrancar,
por eso el contenedor debe quedarse en root.

### 4. El dominio usa el puerto del contenedor, no del host

No elijas un puerto del host: Traefik enruta por la red interna de Docker y las
apps existentes dejan `ports` vacío. El `port` del dominio es el **puerto del
contenedor** (3000 para GOWA, igual que catalogqr y hlgcodes-web). Por eso los
puertos 3000/3001/8080 del host no estorban.

`https` + `certificateType: letsencrypt` funciona sin tocar nada porque
`*.hlgcodes.com` ya resuelve ( comodín) y Let's Encrypt hace HTTP-01.

## Lectura de logs

El `errorMessage` del deployment viene `null` casi siempre; el detalle está en
un archivo del servidor:

```sh
sudo tail -40 /etc/dokploy/logs/<appName>/<appName>-<fecha>.log
```

Para capturar el comando real de build (muy útil para diagnosing contextos):

```sh
# en una terminal, antes de lanzar el deploy
for i in $(seq 1 400); do
  sudo ps ax -o args= | grep -E 'buildx|docker build' | grep -v grep >> /tmp/watch.log
  sleep 0.1
done
```

## El despliegue de GOWA (referencia)

| Campo | Valor |
|---|---|
| `applicationId` | `Aox0DcLKQEbuvN8_7b83s` |
| `appName` | `gowa-p3c7ca` |
| Repo / rama | `HlgCodes/go-whatsapp-web-multidevice` @ `main` |
| `dockerfile` | `./docker/golang.Dockerfile` |
| `buildPath` / `dockerContextPath` | `"/"` / `"."` |
| `githubId` | `Nh_wpxqWEjqWhm-cXlbac` |
| Dominio | `gowa.hlgcodes.com` (container port 3000, Let's Encrypt) |
| Montajes | `/opt/apps/gowa/{storages,statics}` → `/app/{storages,statics}` |
| Memoria | 512 MiB reserva / 1 GiB límite |
| CPU | sin límites (ver trampa 2) |
| `autoDeploy` | `false` |

`githubId` es obligatorio: sin él el deploy falla en ~1 s. Es la GitHub App de
Dokploy y debe tener acceso al repo (se comprueba con
`github.getGithubRepositories`).

`autoDeploy` en `false` a propósito: un push a la rama no debe reiniciar el
número de ventas sin que alguien lo decida.

### Verificación tras desplegar

```sh
curl -s -o /dev/null -w '%{http_code}\n' https://gowa.hlgcodes.com/          # 401
curl -s -u '<user>:<pass>' https://gowa.hlgcodes.com/app/devices             # JSON
sudo ls -la /opt/apps/gowa/storages/     # whatsapp.db, chatstorage.db, ui/
sudo docker top <contenedor> -eo user,args   # debe ser 20001, no root
```

El nombre real del contenedor lleva sufijo de Swarm
(`gowa-p3c7ca.1.<hash>`), así que `docker inspect gowa-p3c7ca` no funciona:
fíltralo con `docker ps --filter name=gowa`.

Comprobar el MCP (debe devolver `serverInfo` con la versión de `AppVersion`):

```sh
curl -s -u '<user>:<pass>' -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -X POST https://gowa.hlgcodes.com/mcp \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{
       "protocolVersion":"2024-11-05","capabilities":{},
       "clientInfo":{"name":"probe","version":"1"}}}'
```

Un `GET /mcp` que devuelve 400 es lo esperado: el adaptor de Fiber solo monta
POST y DELETE.

## Mantenimiento del host

Dokploy deja imágenes y capas viejas en cada redespliegue:

```bash
docker system prune -af --volumes=false
docker image prune -a -f --filter "until=720h"
```

Nunca `--volumes` en este host compartido.

Ojo con `Chatwoot Community Edition` si se autoaloja aquí: es un Postgres y un
Redis más, y va a comer bastante memoria en una instancia de 12 GB. Ver el
proyecto hermano de infraestructura para Chatwoot.