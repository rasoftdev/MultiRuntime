<p align="center">
  <img src="MultiRuntime.webp" alt="MultiRuntime: múltiples versiones de PHP en un solo servidor (PHP 7.4, 8.1, 8.3, 8.5)" width="100%">
</p>

<p align="center"><a href="README.en.md">English</a> · <b>Español</b></p>

# MultiRuntime (`mrt`)

Gestor de versiones de runtimes para Linux, al estilo `nvm`, empezando por **PHP**.
Un solo binario en Go. Te permite tener tu PHP más reciente en el sistema y, a la vez,
usar PHP 7.4 (u otra versión) en proyectos concretos, con su propio `composer` y sus
propias extensiones. No usa Docker ni toca los paquetes del sistema.

```sh
cd ~/proyectos/laravel-viejo
mrt php local 7.4        # escribe .php-version
php -v                   # PHP 7.4.x
composer install         # corre con PHP 7.4

cd ~/proyectos/nuevo
php -v                   # tu PHP del sistema (8.x)
```

## Contenido

1. [Qué necesitas (requisitos)](#1-qué-necesitas-requisitos)
2. [Instalar mrt](#2-instalar-mrt)
3. [Instalar y configurar una versión de PHP](#3-instalar-y-configurar-una-versión-de-php)
4. [Apuntar un proyecto a una versión de PHP](#4-apuntar-un-proyecto-a-una-versión-de-php)
5. [Extensiones y paquetes adicionales](#5-extensiones-y-paquetes-adicionales)
6. [Mantener todo actualizado](#6-mantener-todo-actualizado)
7. [Referencia de comandos](#7-referencia-de-comandos)
8. [Cómo funciona](#8-cómo-funciona)
9. [Estado y límites](#9-estado-y-límites)
10. [Estructura del proyecto y desarrollo](#10-estructura-del-proyecto-y-desarrollo)

---

## 1. Qué necesitas (requisitos)

Lo que hace falta depende de **qué vas a hacer**. No todo el mundo necesita un compilador.

| Para… | Necesitas | Notas |
|---|---|---|
| **Instalar `mrt`** | Linux (amd64 o arm64; WSL sirve), `bash`, `git`, `curl` | Sin `sudo`: usa tu usuario normal. **No hace falta tener Go**: si no está, se descarga uno temporal y se borra. |
| **Instalar un PHP precompilado** (8.0 en adelante) | Nada más que conexión a internet | Binario estático: se instala en segundos. Suele ir uno o dos parches por detrás de php.net. |
| **Compilar PHP desde fuente** (obligatorio para 7.4; recomendado para el último parche de 8.x) | `gcc`, `make`, `perl`, `pkg-config`, `tar`, `xz` | `mrt` lo comprueba antes de empezar y te dice qué falta. CMake no hace falta: se descarga uno propio. |
| **Añadir extensiones** (`mrt php ext add`) | Lo anterior + `autoconf` y `m4` | Solo en versiones compiladas desde fuente. |
| **Composer** | Nada | Se descarga solo la primera vez, verificado con SHA256, y corre con el PHP del proyecto. |

Instalar las herramientas de compilación (una sola vez por servidor, requiere `sudo`):

```sh
# Debian / Ubuntu
sudo apt install build-essential perl pkg-config xz-utils autoconf m4

# Fedora / RHEL
sudo dnf install gcc gcc-c++ make perl pkgconf-pkg-config xz autoconf m4

# Alpine
sudo apk add build-base perl pkgconf xz autoconf m4
```

### ¿Qué camino me conviene?

| Quiero… | Comando | Cómo se obtiene | Tiempo | Necesita compilador |
|---|---|---|---|---|
| PHP 8.x rápido, aunque sea un parche por detrás | `mrt php install 8.3 --prebuilt` | binario estático | segundos | no |
| El **último** parche de una 8.x | `mrt php install 8.3` | compila desde fuente | minutos | sí |
| **PHP 7.4** | `mrt php install 7.4` | compila desde fuente | minutos | sí |
| Extensiones que no vienen incluidas | `mrt php ext add redis` | compila con `phpize` | minutos | sí (+ `autoconf`, `m4`) |

`mrt php install <versión>` elige siempre el **parche más alto disponible** entre todas las fuentes. Por eso,
sin `--prebuilt`, instalar una 8.x puede compilar desde fuente si php.net tiene un parche más nuevo que el
binario estático; si faltan las herramientas de compilación, se detiene con un mensaje claro y no instala nada.

---

## 2. Instalar mrt

### Opción A: un solo comando (recomendada)

Clona el repositorio en una carpeta temporal, compila `mrt`, lo instala en `~/.mrt` y **borra todo lo
temporal** al terminar, incluso si algo falla.

```sh
curl -fsSL https://raw.githubusercontent.com/rasoftdev/MultiRuntime/main/get.sh | bash
```

Variantes:

```sh
# además añade la línea del PATH a ~/.bashrc y ~/.zshrc
curl -fsSL https://raw.githubusercontent.com/rasoftdev/MultiRuntime/main/get.sh | bash -s -- --write-rc

# actualizar mrt: volver a ejecutar el mismo comando (conserva las versiones de PHP instaladas)

# desinstalar: borra ~/.mrt entero (incluidas las versiones de PHP) y las líneas del rc
curl -fsSL https://raw.githubusercontent.com/rasoftdev/MultiRuntime/main/get.sh | bash -s -- uninstall
```

Variables opcionales: `MRT_REPO` (URL git), `MRT_REF` (rama o tag, por defecto `main`) y `MRT_HOME`
(carpeta de instalación, por defecto `~/.mrt`). El script se niega a correr como root. Como con cualquier
`curl | bash`, revísalo antes si no confías en el origen.

> La URL asume que el repositorio está en `github.com/rasoftdev/MultiRuntime` (rama `main`). Si lo
> publicas en otra ubicación, cambia la URL del comando o define `MRT_REPO`.

### Opción B: desde el repositorio clonado

```sh
scripts/build.sh                  # compila dist/mrt-linux-amd64 y dist/mrt-linux-arm64 (requiere Go 1.22+)
scripts/install.sh --write-rc     # instala en ~/.mrt y añade la línea al .bashrc/.zshrc
```

También se puede instalar desde una URL con un binario ya compilado:
`MRT_URL=https://host/mrt-linux-amd64 scripts/install.sh`.

### Activar los atajos en tu shell

Si no usaste `--write-rc`, añade esta línea a tu `~/.bashrc` o `~/.zshrc` y abre una shell nueva:

```sh
eval "$(~/.mrt/bin/mrt env)"
```

Esto pone `~/.mrt/shims` **antes** de tu PHP del sistema en el `PATH`. Sin ningún `.php-version`, los atajos
`php` y `composer` pasan directamente al PHP del sistema, así que no cambia nada hasta que lo pidas.

### Comprobar la instalación

```sh
mrt doctor
```

Debe marcar `ok` los atajos `php` y `composer` y que los shims están en el `PATH`.

---

## 3. Instalar y configurar una versión de PHP

Ejemplo completo para un proyecto Laravel con PHP 8.3 (el mismo flujo sirve para cualquier versión):

```sh
# 1) (solo si vas a compilar) instala las herramientas — ver sección 1
sudo apt install build-essential perl pkg-config xz-utils autoconf m4

# 2) instala la versión
mrt php install 8.3 --prebuilt      # rápido, binario estático
#   o: mrt php install 8.3          # último parche, compila desde fuente

# 3) apunta el proyecto a esa versión
cd /ruta/a/tu-proyecto
mrt php local 8.3                   # crea .php-version

# 4) comprueba
php -v                              # PHP 8.3.x
mrt php current                     # versión en uso y de dónde sale
php -m | grep -iE 'mbstring|openssl|pdo|tokenizer|xml|ctype|fileinfo|curl|zip|gd|intl'

# 5) instala las dependencias
composer install                    # la primera vez descarga Composer
php artisan --version
```

Notas:

- **Comprueba siempre `php -m`.** Laravel necesita como mínimo `mbstring`, `openssl`, `pdo`, `tokenizer`,
  `xml`, `ctype` y `fileinfo`. Si falta una extensión en un binario estático no se puede añadir con
  `ext add`: reinstala la versión compilada desde fuente con `mrt php install 8.3 --profile source`
  (ver [sección 5](#5-extensiones-y-paquetes-adicionales)).
- **La configuración de PHP** (`php.ini`) de cada versión está en `~/.mrt/php/versions/<x.y.z>/etc/php.ini`,
  con extras en `etc/conf.d/*.ini`. Cada versión tiene el suyo; editar uno no afecta a las demás.
  Valores por defecto: `memory_limit = 1G`, `date.timezone = UTC`.
- **Servidor web (nginx / apache con php-fpm):** `mrt` gestiona la línea de comandos. El servidor web no
  pasa por los atajos. Las versiones compiladas desde fuente incluyen `php-fpm`
  (`~/.mrt/php/versions/<x.y.z>/sbin/php-fpm`), pero esa ruta no está probada; si tu servidor ya tiene
  php-fpm del sistema, el sitio sigue usándolo y `mrt` solo cambia la CLI.
- **Cron, supervisor, systemd, CI:** ahí no existe el `PATH` de tu shell. Usa la ruta completa y ejecuta
  desde la raíz del proyecto para que encuentre el `.php-version`:
  ```sh
  cd /var/www/mi-proyecto && ~/.mrt/shims/php artisan schedule:run
  ```
- **Primera compilación desde fuente:** es bastante más lenta porque compila todas las librerías propias
  (OpenSSL 1.1, ICU, libxml2…). Se guardan en `~/.mrt/deps/` y las siguientes versiones las reutilizan; en
  WSL, PHP 7.4 con las librerías ya hechas tardó unos 4,5 minutos. Si falla, el registro completo está en
  `~/.mrt/cache/build/<versión>/build.log`. Al volver a ejecutar el comando, las librerías que ya
  terminaron no se recompilan.

---

## 4. Apuntar un proyecto a una versión de PHP

La versión de un proyecto se define con un archivo `.php-version` en su raíz: texto plano con la versión,
igual que en `nvm`/`pyenv`.

```sh
cd /ruta/a/mi-proyecto
mrt php install 7.4        # una sola vez por máquina (si no está instalada)
mrt php local 7.4          # crea .php-version con "7.4"
php -v                     # PHP 7.4.x
composer install           # corre con PHP 7.4
```

`mrt php local <versión>` equivale a crear el archivo a mano: `echo "7.4" > .php-version`.

**Formatos aceptados**

| Contenido | Significa |
|---|---|
| `8` | el último PHP 8.x instalado |
| `8.3` | el último parche 8.3.x instalado (recomendado: sigue los parches de seguridad) |
| `8.3.12` | exactamente esa versión |
| `system` | ignorar mrt y usar el PHP del sistema (solo escribiéndolo a mano en el archivo) |

Se permiten líneas vacías y comentarios que empiecen por `#`; solo cuenta la primera línea con contenido.

**Reglas**

- `mrt` busca `.php-version` en el directorio actual y **sube por los directorios padre**, así que
  funciona desde cualquier subcarpeta del proyecto (`app/`, `storage/`, etc.).
- Si hay varios, gana el más cercano. Un monorepo puede tener un `.php-version` distinto por carpeta.
- Si el proyecto pide una versión que no está instalada, `php` y `composer` se detienen con
  `Run: mrt php install <versión>`; no usan otra por su cuenta.
- Sin `.php-version` se usa la versión global (`mrt php global 8.3`) y, si no hay ninguna, el PHP del sistema.
- Commitea `.php-version` al repositorio: todo el equipo y el servidor usan la misma versión.

**Orden de prioridad completo**

1. Variable `MRT_PHP_VERSION` (solo para ese comando o sesión)
2. `.php-version` más cercano, subiendo desde el directorio actual
3. Versión global (`mrt php global <versión>`)
4. PHP del sistema

```sh
MRT_PHP_VERSION=8.3 php -v           # prueba puntual sin tocar el proyecto
mrt php current                      # qué versión se usa aquí y de dónde viene
mrt php which                        # ruta del binario en uso
```

**Cambiar de versión más adelante:** `mrt php local 8.3` sobrescribe el archivo. Después conviene volver a
resolver las dependencias (`composer install`, o `composer update` si tu `composer.json` lo permite).

---

## 5. Extensiones y paquetes adicionales

Las versiones **compiladas desde fuente** incluyen: `bcmath calendar ctype curl dom exif fileinfo filter ftp
gd gettext hash iconv intl json libxml mbstring mysqli mysqlnd opcache openssl pcntl pdo pdo_mysql pdo_pgsql
pdo_sqlite pgsql phar posix readline session shmop simplexml soap sockets sodium sqlite3 sysvmsg sysvsem
sysvshm tokenizer xml xmlreader xmlwriter xsl zip zlib`, más `php-fpm`.

Para añadir más después (con la aplicación ya instalada y configurada), usa `mrt php ext`:

```sh
mrt php ext add redis              # PECL: elige el último release estable compatible con tu PHP
mrt php ext add xdebug@3.1.6       # o fija una versión concreta
mrt php ext add gmp                # incluida en el código de PHP (compila también su librería)
mrt php ext list                   # extensiones activadas por .ini
mrt php ext remove redis
mrt php ext add redis --php 7.4    # sobre otra versión instalada, no la del directorio actual
```

Cómo funciona: descarga el código (de PECL o del propio PHP), lo compila con `phpize` contra **esa** versión
de PHP, instala el `.so` y deja un `.ini` activado en el `conf.d` de esa versión. No toca las demás.

Límites:

- Solo funciona en versiones **compiladas desde fuente** (necesita `autoconf` y `m4`, ver sección 1). Los
  binarios estáticos no pueden cargar extensiones externas: para tener una versión ampliable, instálala con
  `mrt php install <versión> --profile source`.
- Si una extensión necesita una librería del sistema que `mrt` no compila (por ejemplo `imagick` necesita
  ImageMagick), la compilación fallará; instala esa librería en el sistema o elige otra extensión.

---

## 6. Mantener todo actualizado

```sh
mrt php outdated            # líneas instaladas (7.4, 8.3…) con un parche más nuevo disponible
mrt php update              # instala el último parche de cada línea instalada
mrt php update 8.3          # solo esa línea
mrt php update --prune      # además borra los parches antiguos de la misma línea
mrt php list                # qué hay instalado
mrt php uninstall 8.3.12    # borra una versión exacta
```

Con un `.php-version` del tipo `8.3` (sin parche), el proyecto pasa solo al parche nuevo en cuanto
está instalado. Si fijaste un parche exacto (`8.3.12`) y lo borras con `--prune`, ese proyecto dará error
hasta que lo cambies. Para actualizar `mrt` en sí, vuelve a ejecutar el comando de instalación.

---

## 7. Referencia de comandos

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
| `mrt php outdated` | líneas instaladas con un parche más nuevo |
| `mrt php update [spec] [--prune]` | instala el último parche de cada línea instalada |
| `mrt php uninstall <x.y.z>` | borra una versión |
| `mrt php ext add\|list\|remove` | gestiona extensiones de la versión activa (`--php x.y.z` para otra) |

Opciones de instalación:

- `--prebuilt`: acepta el binario precompilado más nuevo aunque php.net tenga un parche posterior.
- `--profile source`: fuerza compilar desde las fuentes de php.net.
- `--profile bulk|common|minimal`: conjunto de extensiones del binario estático (por defecto `bulk`, el más completo).

Variables de entorno: `MRT_HOME` (por defecto `~/.mrt`), `MRT_PHP_VERSION`.

---

## 8. Cómo funciona

- `mrt setup` copia el binario a `~/.mrt/bin/mrt` y crea los atajos `php` y `composer` en `~/.mrt/shims/`
  (enlaces simbólicos a `mrt`; el comportamiento depende del nombre con el que se invoca).
- Al ejecutar `php` o `composer`, `mrt` resuelve la versión (ver [sección 4](#4-apuntar-un-proyecto-a-una-versión-de-php))
  y ejecuta ese binario con su propio `php.ini` y `conf.d/`.
- `composer` se descarga como phar (verificado con SHA256) a `~/.mrt/tools/composer/` y se ejecuta con el
  PHP del proyecto. Para PHP anterior a 7.2.5 usa la línea Composer 2.2 LTS.

Disposición en disco:

```
~/.mrt/
├── bin/mrt                     el binario
├── shims/{php,composer}        enlaces a mrt (van en el PATH)
├── php/versions/<x.y.z>/       cada versión: bin/, etc/php.ini, etc/conf.d/, lib/
├── php/global                  versión global
├── deps/v1/                    librerías compiladas, compartidas entre versiones
├── tools/                      composer.phar, cmake
└── cache/                      descargas y compilaciones en curso
```

### De dónde sale cada PHP

`mrt php install` consulta todas las fuentes y elige el **parche más alto**; con la misma versión en
ambas, prefiere el binario precompilado por rapidez.

1. **Binarios estáticos** de [static-php-cli](https://dl.static-php.dev): instalación en segundos, sin
   dependencias. Solo existen desde PHP 8.0 y a veces van unos parches por detrás de php.net. No cargan
   extensiones externas.
2. **Compilación desde las fuentes de php.net** (PHP 7.4 a 8.5): es la única vía para 7.4. Se verifica el
   SHA256 publicado por php.net. Las librerías se compilan una vez en `~/.mrt/deps/` y se comparten:
   OpenSSL 1.1.1w, libxml2 2.9.14, libxslt, oniguruma, SQLite, curl, libpng, libjpeg-turbo, freetype,
   libzip, ICU 72, libsodium, ncurses, readline y libpq. Así el resultado no depende de las versiones de
   librerías de tu distro (por ejemplo, PHP 7.4 no funciona con OpenSSL 3).

---

## 9. Estado y límites

Probado en Ubuntu 26.04 (WSL, amd64, GCC 15):

- Binario estático PHP 8.1.34 + Composer 2.11.2 con `.php-version`, y vuelta al PHP 8.3 del sistema fuera del proyecto.
- PHP 7.4.33 compilado desde fuente con 53 módulos, entre ellos `gd` (png, jpeg, freetype), `zip`, `intl`
  (ICU 72.1), `sodium`, `readline`, `pgsql`/`pdo_pgsql`, `xsl`, `soap`; HTTPS y curl con OpenSSL 1.1.1w.
- `mrt php ext add redis` (PECL) y `mrt php ext add gmp` (incluida en PHP, con su librería): compilan, se
  activan y cargan; `ext list` funciona.
- Laravel 8.83 sobre PHP 7.4.33: `composer create-project`, `php artisan` y `composer install` desde un
  `composer.lock`; `php` sigue resolviendo 7.4 desde subdirectorios del proyecto.
- `get.sh` completo contra un repositorio local: descarga, Go temporal, instalación, reejecución como
  actualización, desinstalación y limpieza de temporales.

**Sin verificar** (comprueba el resultado antes de depender de ello):

- Compilar versiones 8.x desde fuente (solo se ha probado 7.4).
- `mrt php update` con algo realmente desactualizado (solo se vio el caso "todo al día"), `ext remove`,
  `php-fpm` y los flags `--prune` / `--prebuilt`.
- Qué extensiones trae exactamente el binario estático `bulk` (compruébalo con `php -m`).
- arm64: solo se comprobó que `mrt` compila para esa arquitectura, no que se ejecute.
- Otras distros además de Ubuntu (Debian, Fedora, Alpine).
- `get.sh`: la negativa a correr como root y la opción `--write-rc`.

Nota sobre Composer: desde la 2.9, `composer require/update/create-project` bloquea por defecto los paquetes
con avisos de seguridad. En proyectos antiguos (p. ej. Laravel 8) puede impedir resolver dependencias; se
desactiva con `COMPOSER_NO_SECURITY_BLOCKING=1` o `config.audit.block-insecure: false` en `composer.json`.
`composer install` desde un `composer.lock` existente no se ve afectado.

Límites conocidos:

- Solo Linux (WSL incluido). No hay soporte nativo para Windows ni macOS.
- Compilación desde fuente: solo 7.4 a 8.5. PHP 7.3 o anterior no está soportado.
- Los binarios estáticos no admiten `ext add`; para eso, instala con `--profile source`.
- Las rutas de compilación quedan fijadas: si mueves `MRT_HOME`, reinstala las versiones.
- Los checksums de las librerías compiladas están fijados en el código, obtenidos al descargarlas por primera
  vez (coinciden con los publicados oficialmente en los casos que se pudo contrastar); los binarios
  estáticos no tienen checksum publicado.
- OpenSSL 1.1.1 está sin soporte upstream; se usa solo dentro de estas instalaciones privadas, por
  compatibilidad con PHP 7.4.

---

## 10. Estructura del proyecto y desarrollo

```
cmd/mrt/               CLI (comandos, despacho por nombre de atajo)
internal/config/       rutas de ~/.mrt
internal/versions/     parseo y comparación de versiones
internal/fetch/        descargas, SHA256, extracción tar.gz
internal/phpruntime/   proveedores (static, source), instalación, resolución,
                       Composer, extensiones, update
internal/shim/         lógica de los atajos php y composer
scripts/               build.sh, install.sh
get.sh                 instalador remoto (curl | bash)
```

Para añadir otro runtime (Node, Python…) la idea es crear otro paquete junto a `phpruntime` con su propia
lista de proveedores y registrar su atajo en `internal/shim`. La arquitectura de proveedores
(`Provider`: `List` + `Install`) está pensada para eso.

```sh
go vet ./...
go build ./cmd/mrt
scripts/build.sh
```

---

## Código abierto

MultiRuntime es software libre y de código abierto, publicado bajo la **licencia MIT** (ver [LICENSE](LICENSE)):
puedes usarlo, copiarlo, modificarlo y distribuirlo, también en proyectos comerciales, conservando el aviso de
copyright.

**Contribuir.** Las contribuciones son bienvenidas:

1. Abre un *issue* con el error o la idea (incluye la salida de `mrt doctor` y `mrt php current`, tu distro y arquitectura).
2. Para cambios de código, haz un *fork*, crea una rama y abre un *pull request*.
3. Antes de enviarlo: `gofmt -l .` sin salida, `go vet ./...` limpio y `scripts/build.sh` compilando para amd64 y arm64.
4. Si cambias el comportamiento o los comandos, actualiza **los dos README** (`README.md` y `README.en.md`).

Especialmente útiles: pruebas en otras distros (Debian, Fedora, Alpine) y en arm64, compilar versiones 8.x desde
fuente y tests automáticos (ver [Estado y límites](#9-estado-y-límites)).

**Software de terceros.** `mrt` descarga y usa software ajeno, cada uno con su propia licencia: PHP (PHP License),
Composer (MIT), OpenSSL, libxml2, curl, ICU, SQLite y el resto de librerías que compila, además de los binarios de
[static-php-cli](https://github.com/crazywhalecc/static-php-cli). La licencia MIT de este repositorio cubre solo el
código de `mrt`, no ese software.

## Autor y contacto

Creado por **Ricardo Alvarez** ([RASOFTDEV](https://github.com/rasoftdev)).

- Correo: [dev@ricardoalvarez.co](mailto:dev@ricardoalvarez.co)
- Web: [me.ricardoalvarez.co](https://me.ricardoalvarez.co)

¿Encontraste un error o tienes una idea? Escríbeme por correo.
