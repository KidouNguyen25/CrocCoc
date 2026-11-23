// Produce a CycloneDX SBOM from the Go module metadata embedded in the
// *built binaries*, including the privately built Croc worker dependency graph.
// No network access or extra npm/Go generator dependency is required.
import { execFileSync } from 'node:child_process'
import { writeFileSync } from 'node:fs'
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

for (const [name, file] of executables) {
  const buildInfo = execFileSync('go', ['version', '-m', file], {
    encoding: 'utf8',
    maxBuffer: 16 * 1024 * 1024,
  })
  let found = 0
  for (const line of buildInfo.split(/\r?\n/)) {
    const fields = line.trim().split(/\t+/)
    if (fields[0] !== 'dep' || !fields[1] || !fields[2]) continue
    const [, moduleName, version] = fields
    if (!version.startsWith('v')) continue
    found += 1
    const key = `${moduleName}@${version}`
    const existing = modules.get(key)
    if (existing) {
      existing.usedBy.add(name)
    } else {
      modules.set(key, { moduleName, version, usedBy: new Set([name]) })
    }
  }
  if (found === 0) {
    throw new Error(`Go build metadata missing dependencies for ${name}; SBOM would be incomplete`)
  }
}

if (modules.size === 0) {
  throw new Error('No Go dependency metadata found in either executable; SBOM is incomplete')
}

const components = [...modules.values()]
  .sort((a, b) => a.moduleName.localeCompare(b.moduleName) || a.version.localeCompare(b.version))
  .map(({ moduleName, version, usedBy }) => ({
    type: 'library',
    name: moduleName,
    version,
    purl: `pkg:golang/${moduleName.split('/').map(encodeURIComponent).join('/')}@${encodeURIComponent(version)}`,
    properties: [{
      name: 'croccoc:embedded-in',
      value: [...usedBy].sort().join(', '),
    }],
  }))

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
