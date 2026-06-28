# Bundling Graphviz with VirtBBS

Network topology diagrams need Graphviz's `dot` binary. VirtBBS looks for it in this order:

1. **`paths.graphviz_dot`** in `VirtBBS.DAT` (optional explicit path)
2. **Bundled copy** next to the server: `graphviz/bin/dot` (or `graphviz/bin/dot.exe` on Windows)
3. **`dot` on PATH** (system install)

## Bundle for your install directory

From the directory that contains `virtbbs` (or your release tree):

```bash
chmod +x scripts/bundle-graphviz.sh   # once, from the repo checkout
./scripts/bundle-graphviz.sh .
```

This creates:

```
graphviz/
  bin/dot          # or dot.exe
  lib/             # platform shared libraries (when needed)
```

Ship **`virtbbs` + `graphviz/`** together in your install or release tarball. The bundled tree is gitignored — run the script on each target platform when packaging.

**macOS / Linux:** install Graphviz once on the build machine (`brew install graphviz`, `apt install graphviz`, etc.), then run the script.

**Windows:** install Graphviz, then run the script from Git Bash or set `GRAPHVIZ_PREFIX` to the Graphviz install root before running.

```bash
GRAPHVIZ_PREFIX=/opt/homebrew/opt/graphviz ./scripts/bundle-graphviz.sh /opt/virtbbs
```

## Verify

```bash
./graphviz/bin/dot -V
./virtbbs -config VirtBBS.DAT
# After nodelist import or rebuild maps, check logs for diagram output.
```

If bundled `dot` fails with a missing-library error on Linux, ensure `graphviz/lib/` was populated (the script uses `ldd` to copy dependencies).
