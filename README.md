<div align="center">

# ⚡ GOENMA TRAFTEST (`traftest`)
### Simulador de Tráfico Web y Pruebas de Rendimiento Local sin Fricción

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey.svg)]()
[![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen.svg)]()

**Parte del ecosistema de herramientas para desarrolladores de GOENMA.**

<br />

> **¿Qué es TrafTest?**  
> **TrafTest** (Traffic + Test) es una herramienta ligera y ultra rápida diseñada para que cualquier desarrollador pueda probar la velocidad, estabilidad y resistencia de sus APIs o páginas web locales **en menos de 5 segundos**, sin necesidad de configurar herramientas complejas como JMeter o k6.

<br />

</div>

---

## 🎯 ¿Por qué usar TrafTest?

Cuando creamos una API o aplicación web, suele funcionar bien con 1 usuario (nosotros mismos probando en el navegador). Pero... **¿qué pasa si 20 o 50 usuarios hacen peticiones al mismo tiempo?**

Aquí es donde entra **TrafTest**:
1. **Doble experiencia visual**:
   - 🌐 **Dashboard Web en Vivo:** Abre una página en tu navegador con gráficos en tiempo real de velocidad y latencia.
   - 🖥️ **Modo Consola (TUI):** Una interfaz elegante dentro de tu propia terminal (ideal para servidores o SSH).
2. **Cero configuraciones:** No necesitas crear archivos de scripts complejos en JavaScript ni XML. Un solo comando y listo.
3. **Control total de tráfico:** Puedes controlar cuántas peticiones por segundo enviar o simular múltiples usuarios en paralelo.
4. **Reportes instantáneos:** Exporta reportes en formato **Markdown (`.md`)** o **JSON** con un solo clic para documentar tu proyecto o compartir con tu equipo.

---

## 📦 Instalación Rápida

### Opción 1: Con Go instalado en tu equipo
```bash
go install github.com/eyejdev/traftest@latest
```

### Opción 2: Descargar el ejecutable directo (Sin instalar Go ni nada adicional)
1. Ve a la sección de **[Releases](https://github.com/eyejdev/traftest/releases)**.
2. Descarga el archivo para tu sistema operativo:
   - **Windows:** `traftest-windows-amd64.exe` (renómbralo a `traftest.exe`).
   - **macOS:** `traftest-darwin-arm64` (Apple Silicon M1/M2/M3) o `traftest-darwin-amd64` (Intel).
   - **Linux:** `traftest-linux-amd64` o `traftest-linux-arm64`.

---

## 🚀 Guía Práctica para Principiantes

### 🔰 1. ¿No tienes una API lista? Prueba con un servidor público de prueba
Si estás empezando y solo quieres ver cómo funciona la herramienta de inmediato:
```bash
traftest https://jsonplaceholder.typicode.com/posts/1 -c 5 -r 10 -d 10s
```
*Esto enviará 10 peticiones por segundo durante 10 segundos a un servidor público de pruebas y abrirá tus gráficos en vivo.*

---

### 🚀 2. Tu primera prueba sobre tu API local (Modo Visual Web)
Simplemente pasa la dirección de tu aplicación local:
```bash
traftest http://localhost:8080/api/productos
```
*Se abrirá automáticamente tu navegador con un dashboard oscuro mostrando gráficos de peticiones por segundo y tiempos de respuesta.*

---

### ⏱️ 3. Simular tráfico constante durante un tiempo
Supongamos que quieres simular **50 peticiones por segundo** durante **30 segundos** con **10 usuarios simultáneos (workers)**:
```bash
traftest http://localhost:8080/api/usuarios -c 10 -r 50 -d 30s
```
- `-c 10`: 10 conexiones concurrentes (usuarios en paralelo).
- `-r 50`: Límite de 50 peticiones por segundo (RPS).
- `-d 30s`: La prueba terminará automáticamente a los 30 segundos.

---

### 📬 4. Probar peticiones POST con envío de datos (JSON)
Si tienes un endpoint de registro o creación:
```bash
traftest http://localhost:8080/api/crear -X POST -b '{"nombre":"Juan", "rol":"dev"}' -c 5 -n 200
```
- `-X POST`: Método HTTP POST.
- `-b '{...}'`: El contenido JSON que quieres enviar.
- `-n 200`: La prueba se detendrá automáticamente al completar 200 peticiones.

> 💡 *También puedes usar un archivo JSON externo con:* `--body-file ./payload.json`

---

### 🔐 5. Probar endpoints protegidos con Tokens / Headers
Si tu API requiere autenticación mediante Token Bearer o API Keys:
```bash
traftest http://localhost:8080/api/perfil -H "Authorization: Bearer MI_TOKEN_SECRETO" -c 10 -n 500
```

---

### 🖥️ 6. Modo Terminal Interactiva (TUI)
Si prefieres no abrir el navegador y ver todo en tu consola:
```bash
traftest --cli http://localhost:8080/health -c 10 -n 1000 -o reporte.md
```
* **Barra Espaciadora / `p`:** Pausa / Reanuda el envío de tráfico en vivo.
* **`q` / `Ctrl + C`:** Detiene la prueba y genera el resumen final.
* `-o reporte.md`: Guarda automáticamente el informe final en un archivo Markdown.

---

## 📊 ¿Cómo entender los resultados de tu prueba?

TrafTest te mostrará métricas estadísticas clave. Aquí te explicamos qué significan de forma muy sencilla:

| Métrica | ¿Qué significa en lenguaje simple? | ¿Qué valor buscar? |
| :--- | :--- | :--- |
| **Instant RPS** | Peticiones por segundo procesadas en cada instante. | Que se mantenga estable y cercano a lo que configuraste. |
| **p50 (Mediana)** | El **50%** de las peticiones respondieron más rápido que este tiempo. | Es la velocidad que experimenta un usuario promedio (ej. `< 50ms`). |
| **p95** | El **95%** de las peticiones fueron más rápidas que este tiempo. | Te muestra si el sistema empieza a ponerse lento en momentos de estrés. |
| **p99** | El **1%** de las peticiones más lentas. | Sirve para detectar consultas lentas a base de datos o bloqueos del servidor. |
| **HTTP Status 2xx** | Peticiones exitosas. | Debe ser el 100% o muy cercano. |
| **HTTP Status 5xx / Errores** | El servidor arrojó excepciones o rechazó conexiones. | **0%**. Si aparecen errores, el servidor no soportó la carga. |

---

## 🎛️ Tabla Completa de Opciones (Flags)

| Opción | Shorthand | Descripción | Ejemplo | Valor por Defecto |
| :--- | :--- | :--- | :--- | :--- |
| `--url` | `-u` | URL objetivo a probar | `-u http://localhost:3000` | `http://127.0.0.1:8080` |
| `--method` | `-X` | Método HTTP | `-X POST` | `GET` |
| `--concurrency` | `-c` | Número de trabajadores en paralelo | `-c 25` | `10` |
| `--requests` | `-n` | Total de peticiones a enviar | `-n 1000` | `0` (ilimitado) |
| `--duration` | `-d` | Duración del test (`10s`, `1m`, `30s`) | `-d 45s` | `0s` (sin límite de tiempo) |
| `--rps` | `-r` | Límite de peticiones por segundo | `-r 100` | `0` (máximo posible) |
| `--header` | `-H` | Header HTTP (repetible) | `-H "Auth: 123"` | `[]` |
| `--body` | `-b` | Cuerpo de la petición | `-b '{"id":1}'` | `""` |
| `--body-file` | | Archivo con el cuerpo de la petición | `--body-file data.json` | `""` |
| `--timeout` | `-t` | Timeout por petición individual | `-t 5s` | `10s` |
| `--cli` | | Modo terminal interactiva | `--cli` | `false` |
| `--port` | `-p` | Puerto para el dashboard web | `-p 8888` | `9090` |
| `--output` | `-o` | Guardar reporte (`.md` o `.json`) | `-o informe.md` | `""` |
| `--max-p95` | | Fallar (`exit 1`) si p95 supera este valor | `--max-p95 300ms` | `""` |
| `--max-errors` | | Fallar (`exit 1`) si % de error supera límite | `--max-errors 1.5` | `-1` |

---

## 🛠️ Desarrollo y Contribución

Si deseas compilar o contribuir al proyecto:

```bash
# 1. Clonar repositorio
git clone https://github.com/eyejdev/traftest.git
cd traftest

# 2. Ejecutar suite de pruebas
go test -v ./...

# 3. Compilar ejecutable
go build -o traftest main.go
```

---

## 💖 Apoya el Proyecto / Patrocinios

Si **TrafTest** te ahorra tiempo y te resulta útil en tu día a día como desarrollador, puedes apoyar su mantenimiento y la creación de nuevas funciones:

- ⭐ **Danos una Estrella en GitHub:** Ayuda a que más desarrolladores conozcan la herramienta.
- ☕ **Invítanos un café:** Apoya el desarrollo a través de [Buy Me a Coffee (eyejdev)](https://buymeacoffee.com/eyejdev) o mediante el botón **[Sponsor](https://github.com/sponsors/eyejdev)** en la parte superior del repositorio.

<br />

<div align="center">
  <a href="https://buymeacoffee.com/eyejdev" target="_blank">
    <img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" height="42px" width="150px">
  </a>
</div>

---

## 📄 Licencia

Este proyecto está bajo la Licencia **MIT**. Consulta el archivo [LICENSE](LICENSE) para más detalles.

Desarrollado con ❤️ para el ecosistema **GOENMA**.
