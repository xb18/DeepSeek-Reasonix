#!/usr/bin/env node
// Keeps integrations/dsh-plugin/market-index.json, the DeepSeek Harness plugin
// market index for this repository, pinned to the commit that last changed the
// package. Usage: dsh-market-index.mjs --pin | --check
import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

const PKG_DIR = 'integrations/dsh-plugin'
const INDEX = `${PKG_DIR}/market-index.json`
const INDEX_URL = `https://raw.githubusercontent.com/esengine/DeepSeek-Reasonix/studio/${INDEX}`
const VALIDATOR = '@deepseek-ai/dsh-plugin-market-index'

const git = (...args) => execFileSync('git', args, { encoding: 'utf8' }).trim()
const read = (path) => JSON.parse(readFileSync(path, 'utf8'))
const lastPackageCommit = () => git('log', '-1', '--format=%H', '--', `${PKG_DIR}/package.json`, `${PKG_DIR}/cordis.patch.yml`)

function validator(version) {
  const dir = mkdtempSync(join(tmpdir(), 'dsh-market-'))
  execFileSync('npm', ['install', '--prefix', dir, '--silent', `${VALIDATOR}@${version}`], { stdio: 'inherit' })
  return import(pathToFileURL(join(dir, 'node_modules', VALIDATOR, 'lib', 'index.js')).href)
}

const mode = process.argv[2]
const index = read(INDEX)
const [plugin] = index.plugins
const [entry] = plugin.versions

if (mode === '--pin') {
  const pkg = read(`${PKG_DIR}/package.json`)
  const sha = lastPackageCommit()
  entry.version = pkg.version
  entry.engines = { dsh: pkg.engines.dsh }
  entry.publishTimestamp = Number(git('show', '-s', '--format=%ct', sha))
  entry.source.commit = sha
  writeFileSync(INDEX, `${JSON.stringify(index, null, 2)}\n`)
  console.log(`pinned ${pkg.name}@${pkg.version} to ${sha}`)
} else if (mode === '--check') {
  const pkg = read(`${PKG_DIR}/package.json`)
  const sha = entry.source.commit
  const stale = []
  if (entry.version !== pkg.version) stale.push(`version ${entry.version} != package ${pkg.version}`)
  if (entry.engines?.dsh !== pkg.engines.dsh) stale.push(`engines.dsh ${entry.engines?.dsh} != package ${pkg.engines.dsh}`)
  if (sha !== lastPackageCommit()) {
    const moved = git('diff', '--name-only', sha, 'HEAD', '--', `${PKG_DIR}/package.json`, `${PKG_DIR}/cordis.patch.yml`)
    if (moved) stale.push(`pin ${sha} predates changes to: ${moved.split('\n').join(', ')}`)
  }
  const { parseMarketIndex, marketInstallTarget } = await validator(pkg.engines.dsh)
  parseMarketIndex(index, INDEX_URL)
  console.log(`install spec: ${marketInstallTarget(plugin.name, entry).spec}`)
  if (stale.length) {
    console.error(`stale index: ${stale.join('; ')}\nrun: node scripts/dsh-market-index.mjs --pin`)
    process.exit(1)
  }
  console.log('index is valid and pinned to the current package')
} else {
  console.error('usage: dsh-market-index.mjs --pin | --check')
  process.exit(2)
}
