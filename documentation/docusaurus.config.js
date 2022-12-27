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
                editUrl: 'https://github.com/iotaledger/hornet/edit/production/documentation',
                versions: {
                    current: {
                        label: 'IOTA',
                        badge: true
                    },
                },
            }
        ],
    ],
    staticDirectories: [path.resolve(__dirname, 'static')],
};
