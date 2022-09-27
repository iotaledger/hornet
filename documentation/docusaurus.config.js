const path = require('path');

module.exports = {
    plugins: [
        [
            '@docusaurus/plugin-content-docs',
            {
                id: 'hornet',
                path: path.resolve(__dirname, 'docs'),
                routeBasePath: 'hornet',
                sidebarPath: path.resolve(__dirname, 'sidebars.js'),
                editUrl: 'https://github.com/iotaledger/hornet/edit/develop/documentation',
            }
        ],
    ],
    staticDirectories: [path.resolve(__dirname, 'static')],
};
