# Third-party notices

ContextGate uses the following upstream components. Their licenses apply to their respective components. The distribution includes the applicable upstream notices and, where collected for redistribution, unmodified dependency source files under [third_party/licenses](third_party/licenses/).

## Go components

[go-licenses.csv](third_party/go-licenses.csv) records the licenses found for the executable dependency graph using `google/go-licenses/v2 v2.0.1`. Full collected texts are in `third_party/licenses/_go`; the underscore keeps redistributed license/source material outside this module's Go package discovery. Segment's MIT No Attribution license and the vendored Apache Thrift license/notice were copied directly from the pinned modules after the classifier reported them as unknown.

Go runtime notices are included separately. DuckDB 1.5.5 native dependencies have additional upstream notices in `third_party/licenses/duckdb-v1.5.5`; the bindings and PostgreSQL query parser also contain native code. These upstream files are retained without re-licensing.

## Embedded UI

| Package | Version | License |
|---|---|---|
| react | 19.3.0 | MIT |
| react-dom | 19.3.0 | MIT |
| scheduler | 0.28.0 | MIT |
| lucide-react | 0.577.0 | ISC |

Full texts: `third_party/licenses/frontend/`. Build tools such as TypeScript and Vite are not included in the runtime bundle.

## Linux runtime libraries

The Linux archives bundle unmodified Debian GCC runtime libraries (`libstdc++.so.6` and `libgcc_s.so.1`). Their Debian copyright notices, GPLv3 and LGPLv3 texts are supplied under `licenses/gcc-runtime/` in each archive and `/usr/share/doc/mcpdbhub-runtime/` in the image. The notices describe the GCC Runtime Library Exception and upstream source locations. System glibc is supplied by the host operating system.
