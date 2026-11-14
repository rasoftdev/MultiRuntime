# MultiRuntime (`mrt`)

Gestor de versiones de runtimes para Linux, al estilo `nvm`, empezando por **PHP**.
Un solo binario en Go. Te permite tener tu PHP más reciente en el sistema y, a la vez,
usar PHP 7.4 (u otra versión) en proyectos concretos, con su propio `composer` y sus
propias extensiones. No usa Docker ni toca los paquetes del sistema.

```
cd ~/proyectos/laravel-viejo
mrt php local 7.4        # escribe .php-version
php -v                   # PHP 7.4.x
composer install         # corre con PHP 7.4

cd ~/proyectos/nuevo
php -v                   # tu PHP del sistema (8.x)
```

## Cómo funciona

- `mrt setup` copia el binario a `~/.mrt/bin/mrt` y crea los atajos `php` y `composer`
  en `~/.mrt/shims/` (enlaces simbólicos a `mrt`; el comportamiento depende del nombre
  con el que se invoca).
- Al ejecutar `php` o `composer`, `mrt` elige la versión en este orden:
  1. variable de entorno `MRT_PHP_VERSION`
  2. archivo `.php-version` del directorio actual o de un directorio padre
  3. versión global (`mrt php global`)
  4. el **PHP del sistema** (el primero en el `PATH` fuera de los shims)
- `.php-version` acepta `7`, `7.4` o `7.4.33`. Con `7.4` se usa el parche más nuevo instalado.
- Cada versión vive en `~/.mrt/php/versions/<x.y.z>/` con su propio `php.ini` y `conf.d/`.
- `composer` se descarga como phar (con verificación SHA256) a `~/.mrt/tools/composer/`
  y se ejecuta con el PHP del proyecto. Para PHP anterior a 7.2.5 usa la línea Composer 2.2 LTS.

## Instalación

Requisitos: Linux (amd64 o arm64). WSL funciona.

```sh
scripts/build.sh                  # compila dist/mrt-linux-amd64 y dist/mrt-linux-arm64
scripts/install.sh --write-rc     # instala en ~/.mrt y añade la línea al .bashrc/.zshrc
```

Sin `--write-rc`, añade tú esto a tu `~/.bashrc` o `~/.zshrc`:

```sh
eval "$(mrt env)"
```

También puedes instalar desde una URL: `MRT_URL=https://host/mrt-linux-amd64 scripts/install.sh`.
Compilar `mrt` requiere Go (probado con 1.27).

Comprobar la instalación: `mrt doctor`.

## Comandos

| Comando | Qué hace |
|---|---|
| `mrt setup` | instala el binario y los atajos en `~/.mrt` |
| `mrt env` | imprime la línea `export PATH=...` |
| `mrt doctor` | comprueba la instalación |
| `mrt php ls-remote [spec]` | versiones instalables |
| `mrt php install <spec>` | instala el último parche que cumpla `spec` (`8.3`, `7.4`, `8.3.12`) |
| `mrt php list` | versiones instaladas (`*` marca la activa) |
| `mrt php local <spec>` | escribe `.php-version` en el directorio actual |
| `mrt php global <spec\|system>` | versión por defecto |
| `mrt php current` / `which` | qué versión se usa aquí y por qué / ruta del binario |
| `mrt php outdated` | líneas instaladas que tienen un parche más nuevo |
| `mrt php update [spec] [--prune]` | instala el último parche de cada línea instalada; `--prune` borra los parches viejos |
| `mrt php uninstall <x.y.z>` | borra una versión |
| `mrt php ext add <nombre>[@versión]` | compila y activa una extensión (PECL o incluida en PHP) |
| `mrt php ext list` / `ext remove <nombre>` | gestiona extensiones de la versión activa (`--php x.y.z` para otra) |

Opciones de instalación:

- `--profile source`: fuerza compilar desde las fuentes de php.net.
- `--prebuilt`: acepta el binario precompilado más nuevo aunque php.net tenga un parche posterior.
- `--profile bulk|common|minimal`: conjunto de extensiones del binario estático.

Variables: `MRT_HOME` (por defecto `~/.mrt`), `MRT_PHP_VERSION`.

## De dónde sale cada PHP

`mrt php install` consulta todas las fuentes y elige el **parche más alto**; si dos fuentes
ofrecen la misma versión, prefiere el binario precompilado por rapidez.

1. **Binarios estáticos** de [static-php-cli](https://dl.static-php.dev): instalación en segundos,
   sin dependencias. Solo existen desde PHP 8.0 y a veces van unos parches por detrás de php.net.
   No pueden cargar extensiones externas (`.so`).
2. **Compilación desde las fuentes de php.net** (PHP 7.4 a 8.5): es la única vía para 7.4.
   Se verifica el SHA256 publicado por php.net. Las librerías se compilan una vez en
   `~/.mrt/deps/` y se comparten entre versiones: OpenSSL 1.1.1w, libxml2 2.9.14, libxslt,
   oniguruma, SQLite, curl, libpng, libjpeg-turbo, freetype, libzip, ICU 72, libsodium,
   ncurses, readline y libpq. Así el resultado no depende de las versiones de librerías de tu
   distro (por ejemplo, PHP 7.4 no funciona con OpenSSL 3). La primera compilación tarda
   bastante más que las siguientes.

Extensiones de las versiones compiladas: `bcmath calendar ctype curl dom exif fileinfo filter
ftp gd gettext hash iconv intl json libxml mbstring mysqli mysqlnd opcache openssl pcntl pdo
pdo_mysql pdo_pgsql pdo_sqlite pgsql phar posix readline session shmop simplexml soap sockets
sodium sqlite3 sysvmsg sysvsem sysvshm tokenizer xml xmlreader xmlwriter xsl zip zlib`, más
`php-fpm`. Se pueden añadir más con `mrt php ext add` (por ejemplo `redis`, `xdebug`, `gmp`, `bz2`).
Para PECL se elige el último release estable compatible con tu versión de PHP.

### Herramientas necesarias para compilar

`gcc`, `make`, `perl`, `pkg-config`, `tar`, `xz`; y `autoconf` y `m4` para `ext add`.
`mrt` lo comprueba antes de empezar y muestra el comando de instalación.

```sh
# Debian/Ubuntu
sudo apt install build-essential perl pkg-config xz-utils autoconf m4
# Fedora/RHEL
sudo dnf install gcc gcc-c++ make perl pkgconf-pkg-config xz autoconf m4
```

CMake, que necesitan unas pocas librerías, se descarga en una versión fija y no hace falta instalarlo.

## Estado y límites

Probado en Ubuntu 26.04 (WSL, amd64):

- Binario estático PHP 8.1.34 + Composer 2.11.2 con `.php-version`, y vuelta a PHP 8.3 del sistema fuera del proyecto.
- PHP 7.4.33 compilado desde fuente (36 módulos, HTTPS y curl con OpenSSL 1.1.1w), `composer require monolog/monolog:^2.9` funcionando.

Escrito pero **sin verificar de extremo a extremo** (comprueba el resultado antes de depender de ello):

- Las extensiones añadidas después: `gd`, `zip`, `intl`, `sodium`, `readline`, `pgsql`, `xsl`, `soap`, `php-fpm`.
- `mrt php ext add/remove/list`.
- Compilar versiones 8.x desde fuente.
- `mrt php update` / `outdated`.
- arm64: solo se comprobó que `mrt` compila para esa arquitectura.

Límites conocidos:

- Solo Linux (WSL incluido). No hay soporte nativo para Windows ni macOS.
- Compilación desde fuente: solo 7.4 a 8.5. PHP 7.3 o anterior no está soportado.
- Los binarios estáticos no admiten `ext add`; para eso, reinstala con `--profile source`.
- Las rutas de compilación quedan fijadas: si mueves `MRT_HOME`, reinstala las versiones.
- Los checksums de las librerías compiladas están fijados en el código, obtenidos al descargarlas por primera vez (coinciden con los publicados oficialmente en los casos que se pudo contrastar); los binarios estáticos no tienen checksum publicado.
- OpenSSL 1.1.1 está sin soporte upstream; se usa solo dentro de estas instalaciones privadas, por compatibilidad con PHP 7.4.

## Estructura del proyecto

```
cmd/mrt/               CLI (comandos, despacho por nombre de atajo)
internal/config/       rutas de ~/.mrt
internal/versions/     parseo y comparación de versiones
internal/fetch/        descargas, SHA256, extracción tar.gz
internal/phpruntime/   proveedores (static, source), instalación, resolución,
                       Composer, extensiones, update
internal/shim/         lógica de los atajos php y composer
scripts/               build.sh, install.sh
```

Para añadir otro runtime (Node, Python…) la idea es crear otro paquete junto a `phpruntime`
con su propia lista de proveedores y registrar su atajo en `internal/shim`.
La arquitectura de proveedores (`Provider`: `List` + `Install`) está pensada para eso.

## Desarrollo

```sh
go vet ./...
go build ./cmd/mrt
scripts/build.sh
```

No hay tests automáticos todavía.
