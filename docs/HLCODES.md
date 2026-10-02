# Fork HlgCodes — notas operativas

Contenido **específico de este fork**. Vive aquí, y no en el `AGENTS.md` raíz,
a propósito: upstream (`aldinokemal/go-whatsapp-web-multidevice`) también
mantiene el `AGENTS.md` raíz y lo edita con frecuencia. Si el detalle del fork
viviera ahí, cada sync produciría un conflicto. Aquí upstream no llega nunca.

El `AGENTS.md` raíz describe la estructura del proyecto (capas, contratos,
comandos). Este archivo describe **nuestras** diferencias.

## Remotos

| Remoto | Repositorio | Uso |
|---|---|---|
| `origin` | `HlgCodes/go-whatsapp-web-multidevice` | nuestro fork, donde se publica |
| `upstream` | `aldinokemal/go-whatsapp-web-multidevice` | el proyecto original |

La ruta del módulo Go sigue diciendo `github.com/aldinokemal/...` porque **no la
cambiamos**: cambiar el import path obligaría a tocar cada import del proyecto sin
ganancia funcional. Por eso los remotos no coinciden con el módulo, y por eso
`release.yml` y las skills de release siguen apuntando a `aldinokemal`.

## Estrategia de sincronización

**`main` es la rama de integración permanente**: `upstream/main` + nuestros
commits, siempre al día. Es la rama por defecto del repo y la que se despliega.

### Flujo de sync

```sh
git checkout main
git fetch upstream --prune
git log --oneline main..upstream/main      # qué trae upstream
git merge upstream/main                    # resolver conflictos aquí, incremental
# ... commits propios sobre main ...
git push origin main
```

Un merge incremental resuelve **un archivo por vez**. El flujo anterior (ramas
`respaldo-*` y sync manual) llegaba a acumular meses de divergencia: en
octubre2026 `main` iba 140 commits atrás y hubo que resolver todo de una vez.

### Antes de dar el sync por terminado

```sh
go test ./...        # desde src/; el CI de upstream corre lo mismo
git log --oneline upstream/main..main   # solo nuestros commits propios
```

### Ramas de respaldo

`respaldo-*` y los tags `backup-antes-sync-*` se conservan como puntos de
retorno. **No** las borres, no las resetees y no hagas force-push sobre ellas.
Son deliberadamente obsoletas.

### Cuándo revisar

Upstream publica seguido. Comprobar el desfase al menos **una vez por
trimestre**:

```sh
git fetch upstream && git rev-list --count main..upstream/main
```

Si el número es alto (≫20), conviene sincronizar pronto.

## CI: divergencia deliberada

`.github/workflows/build-docker-image.yaml` **no** coincide con upstream y no
debe "arreglarse":

- Publica solo en **GHCR** (`ghcr.io/hlgcodes/...`, en minúsculas porque GHCR las
  exige). No publicamos en Docker Hub porque no tenemos sus secrets.
- Se quitó `arm/v7` del matrix: era lento y Dokploy no lo necesita.
- Las versiones de actions se dejan en las de upstream para no perder fixes de
  seguridad de los runners.

El bloque de comentarios en español al inicio de ese archivo explica cada
desviación. **Consérvalo** al fusionar cambios de CI de upstream.

## Entorno local

Verificado el 01Oct2026 en la máquina de desarrollo:

- La toolchain de Go **no está instalada** (no hay `go`, `gofmt` ni `air`), así
  que los comandos `go test` / `go build` fallan hasta instalarla. Sí están
  `docker`, `git`, `gh`, `gcc` y `ffmpeg`. Instalar Go 1.26 para coincidir con
  `src/go.mod`, o verificar vía `docker compose build`.
- Las funciones de media necesitan `ffmpeg` y las herramientas libwebp
  (`cwebp`, `dwebp`, `webpmux`) para stickers WebP animados. Si faltan, fallan
  en tiempo de request, no de build.
- SSH al servidor de apps es `ubuntu@<ip>`, nunca `root` (lo rechaza).

## Detalle que sigue vivo en upstream

`.gitignore` lista `src/gowa`, `main`, `main.exe` y `bin/`, pero **no**
`src/whatsapp`. El comando documentado `go build -o whatsapp .` desde `src/`
deja un binario sin trackear. Compila en otro sitio o bórralo antes de
commitear.

## Despliegue

La app en producción corre en Dokploy: `https://gowa.hlgcodes.com`.

**Lee [DOKPLOY.md](DOKPLOY.md) antes de tocar el despliegue.** Ahí están la
autenticación de la API (header `x-api-key`, variables de entorno vencidas), las
trampas del build (Dockerfile en subdirectorio, límites de CPU rotos) y la
configuración de referencia de la app.

Estado al 01Oct2026: desplegada y verificada (TLS válido, basic auth activo,
persistencia en `/opt/apps/gowa/{storages,statics}`, MCP respondiendo `v9.5.0`).

## Sistema de referencia

Este fork es el número de ventas de la operación comercial: los envíos deben ser
individuales y responding, nunca automatizados en masa. La librería (WhatsMeow)
no es oficial y viola los ToS de WhatsApp; es un riesgo asumido y documentado.