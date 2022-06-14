const path = require('path');

module.exports = {
    plugins: [
        [
            '@docusaurus/plugin-content-docs',
            {
                id: 'hornet-develop',
                path: path.resolve(__dirname, 'docs'),
                routeBasePath: 'hornet',
                sidebarPath: path.resolve(__dirname, 'sidebars.js'),
                editUrl: 'https://github.com/gohornet/hornet/edit/develop/documentation',
                versions: {
                    current: {
                        label: 'Develop',
                        path: 'develop',
                        badge: true
                    },
                },
            }
        ],
    ],
    staticDirectories: [path.resolve(__dirname, 'static')],
};
