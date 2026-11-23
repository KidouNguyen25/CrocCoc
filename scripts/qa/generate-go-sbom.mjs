// Produce a CycloneDX SBOM from the Go module metadata embedded in the
// *built binaries*, including the privately built Croc worker dependency graph.
// No network access or extra npm/Go generator dependency is required.
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

const [directory, appVersion] = process.argv.slice(2)
if (!directory || !appVersion) {
  throw new Error('Usage: node generate-go-sbom.mjs <staged-release-directory> <app-version>')
}

const stage = resolve(directory)
const executables = [
  ['CrocCoc.exe', join(stage, 'CrocCoc.exe')],
  ['bin/croccoc-engine.exe', join(stage, 'bin', 'croccoc-engine.exe')],
]
const modules = new Map()
const workerScript = readFileSync(new URL('../../backend/engine-spike/worker/build.ps1', import.meta.url), 'utf8')
const upstreamCommit = workerScript.match(/\$upstreamCommit\s*=\s*"([0-9a-f]{40})"/)?.[1]
const workerPatchHash = createHash('sha256')
  .update(readFileSync(new URL('../../backend/engine-spike/croc-v11.6.4-manifest-only.patch', import.meta.url)))
  .digest('hex')
if (!upstreamCommit) throw new Error('Missing pinned Croc upstream commit')

for (const [name, file] of executables) {
  const buildInfo = execFileSync('go', ['version', '-m', file], {
    encoding: 'utf8',
    maxBuffer: 16 * 1024 * 1024,
  })
  let found = 0
  let lastDependency = null
  for (const line of buildInfo.split(/\r?\n/)) {
    const fields = line.trim().split(/\t+/)
    if (fields[0] === '=>' && lastDependency) {
      // go version -m lists module replacements on a following indented line.
      // Record local replacements without leaking absolute build-host paths.
      const entry = modules.get(lastDependency)
      if (entry) {
        const target = fields[1] ?? ''
        entry.replacements.add(target.startsWith('.') || /^[A-Za-z]:[\\/]/.test(target) || target.startsWith('/')
          ? 'local patched source'
          : target)
      }
      continue
    }
    lastDependency = null
    if (fields[0] !== 'dep' || !fields[1] || !fields[2]) continue
    const [, moduleName, version] = fields
    if (!version.startsWith('v')) continue
    found += 1
    const key = `${moduleName}@${version}`
    lastDependency = key
    const existing = modules.get(key)
    if (existing) {
      existing.usedBy.add(name)
    } else {
      modules.set(key, { moduleName, version, usedBy: new Set([name]), replacements: new Set() })
    }
  }
  if (found === 0) {
    throw new Error(`Go build metadata missing dependencies for ${name}; SBOM would be incomplete`)
  }
}

if (modules.size === 0) {
  throw new Error('No Go dependency metadata found in either executable; SBOM is incomplete')
}

// Fail closed even when the Croc module was omitted entirely (for example
// because malformed or development-only build metadata failed version parsing).
// Checking only inside .map() never runs the guard for a missing component.
const sawPatchedCroc = [...modules.values()].some((module) =>
  module.moduleName === 'github.com/schollz/croc/v11' &&
  module.usedBy.has('bin/croccoc-engine.exe') &&
  module.replacements.size > 0)
if (!sawPatchedCroc) {
  throw new Error('Pinned Croc worker module/replacement missing from Go build metadata')
}

const components = [...modules.values()]
  .sort((a, b) => a.moduleName.localeCompare(b.moduleName) || a.version.localeCompare(b.version))
  .map(({ moduleName, version, usedBy, replacements }) => {
    const properties = [{
      name: 'croccoc:embedded-in',
      value: [...usedBy].sort().join(', '),
    }]
    for (const replacement of [...replacements].sort()) {
      properties.push({ name: 'croccoc:go-module-replacement', value: replacement })
    }
    if (moduleName === 'github.com/schollz/croc/v11') {
      if (!usedBy.has('bin/croccoc-engine.exe') || replacements.size === 0) {
        throw new Error('Pinned Croc worker must contain its patched module replacement in build metadata')
      }
      properties.push({ name: 'croccoc:upstream-commit', value: upstreamCommit })
      properties.push({ name: 'croccoc:local-patch-sha256', value: workerPatchHash })
    }
    return {
      type: 'library',
      name: moduleName,
      version,
      purl: `pkg:golang/${moduleName.split('/').map(encodeURIComponent).join('/')}@${encodeURIComponent(version)}`,
      properties,
    }
  })

const sbom = {
  bomFormat: 'CycloneDX',
  specVersion: '1.5',
  version: 1,
  metadata: {
    timestamp: new Date().toISOString(),
    component: { type: 'application', name: 'CrocCoc', version: appVersion },
  },
  components,
}

writeFileSync(join(stage, 'sbom-go.cdx.json'), JSON.stringify(sbom, null, 2) + '\n', 'utf8')
