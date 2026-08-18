#!/usr/bin/env node
/**
 * bump_version.mjs — 每次 SVN 提交前自动将版本号最后一位 +1.
 *
 * 用法: node bump_version.mjs
 *   - 读取 app.go 中 GetVersion() 的 "v0.8.NN" 版本号
 *   - 将最后一位 NN +1 写回 (v0.8.NN → v0.8.(NN+1))
 *   - 打印新版本号, 供提交信息使用
 *
 * 规则(用户强制 2026-08-17): 每次提交 SVN 前必须运行本脚本一次,
 * 版本号随提交自动递增, 当前基准 v0.8.16.
 * 注意: 只有 major.minor 为 0.8 时递增最后一位; 其它格式不动 (安全).
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const appGo = path.join(__dirname, 'app.go');

let src = fs.readFileSync(appGo, 'utf8');

// 匹配 GetVersion() 返回的 "vX.Y.N" 版本字面量
const re = /return\s+"v(\d+)\.(\d+)\.(\d+)"/;
const m = src.match(re);

if (!m) {
  console.error('[bump_version] app.go GetVersion() 未找到 "vX.Y.N" 版本字面量, 未做任何修改');
  process.exit(1);
}

const [, major, minor, patch] = m;
if (major !== '0' || minor !== '8') {
  console.error(`[bump_version] 当前版本 v${major}.${minor}.${patch} 不是 0.8.x, 为安全起见不自动递增`);
  process.exit(0);
}

const next = Number(patch) + 1;
const newVersion = `v${major}.${minor}.${next}`;
src = src.replace(re, `return "${newVersion}"`);
fs.writeFileSync(appGo, src, 'utf8');

console.log(`[bump_version] v${major}.${minor}.${patch} → ${newVersion}`);
