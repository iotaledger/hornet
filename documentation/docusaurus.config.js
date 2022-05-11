const path = require('path');

module.exports = {
    title: 'Hornet',
    url: '/',
    baseUrl: '/',
    themes: ['@docusaurus/theme-classic'],
    themeConfig: {
        navbar: {
            // Workaround to disable broken logo href on test build
            logo: {
                src: 'img/logo/HORNET_logo.svg',
                href: 'https://wiki.iota.org/',
            },
        },
    },
    plugins: [
        [
            '@docusaurus/plugin-content-docs',
            {
                id: 'hornet',
                path: path.resolve(__dirname, 'docs'),
                routeBasePath: 'hornet',
                sidebarPath: path.resolve(__dirname, 'sidebars.js'),
                editUrl: 'https://github.com/iotaledger/hornet/edit/mainnet/',
            }
        ],
    ],
    staticDirectories: [path.resolve(__dirname, 'static')],
};
