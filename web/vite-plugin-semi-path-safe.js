/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import fs from 'fs';
import path from 'path';
import { createRequire } from 'module';
import { compileString, Logger } from 'sass';
import { pathToFileURL } from 'url';

const require = createRequire(import.meta.url);
const { semiThemeLoader } = require('@douyinfe/vite-plugin-semi/lib/semi-theme-loader.js');
const {
  convertMapToString,
  transformPath,
} = require('@douyinfe/vite-plugin-semi/lib/utils.js');

function getNodeModulesRoot(filePath) {
  const normalizedPath = transformPath(filePath);
  const marker = '/node_modules/';
  const markerIndex = normalizedPath.indexOf(marker);
  if (markerIndex === -1) {
    return '';
  }
  return normalizedPath.slice(0, markerIndex + marker.length);
}

export function vitePluginSemiPathSafe(options = {}) {
  return {
    name: 'vite-plugin-semi-path-safe',
    load(id) {
      const filePath = transformPath(id);
      const include = options.include ? transformPath(options.include) : undefined;

      if (!/@douyinfe\/semi-(ui|icons|foundation)\/lib\/.+\.css$/.test(filePath)) {
        return null;
      }

      const scssFilePath = filePath.replace(/\.css$/, '.scss');
      const semiLoaderOptions = {
        name: typeof options.theme === 'string' ? options.theme : options.theme?.name,
        cssLayer: options.cssLayer,
        variables: convertMapToString(options.variables || {}),
        include,
      };
      const originalScssRaw = fs.readFileSync(scssFilePath, 'utf-8');
      const newScssRaw = semiThemeLoader(originalScssRaw, semiLoaderOptions);
      const nodeModulesRoot = getNodeModulesRoot(scssFilePath);

      return compileString(newScssRaw, {
        importers: [
          {
            findFileUrl(url) {
              if (url.startsWith('~')) {
                if (!nodeModulesRoot) {
                  return null;
                }
                return new URL(url.substring(1), pathToFileURL(nodeModulesRoot));
              }

              const resolvedFilePath = path.resolve(path.dirname(scssFilePath), url);
              if (fs.existsSync(resolvedFilePath)) {
                return pathToFileURL(resolvedFilePath);
              }
              return null;
            },
          },
        ],
        logger: Logger.silent,
      }).css;
    },
  };
}
