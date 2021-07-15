const lightCodeTheme = require('prism-react-renderer/themes/github');
const darkCodeTheme = require('prism-react-renderer/themes/dracula');

/** @type {import('@docusaurus/types').DocusaurusConfig} */
module.exports = {
  title: 'Hornet',
  tagline:  'Official IOTA Hornet Software',
  url: 'https://hornet.docs.iota.org/',
  baseUrl: '/hornet/',
  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'throw',
  favicon: '/img/logo/favicon.ico',
  organizationName: 'iotaledger', // Usually your GitHub org/user name.
  projectName: 'Hornet', // Usually your repo name.
  stylesheets: [
    'https://fonts.googleapis.com/css?family=Material+Icons',
  ],
  themeConfig: {
    colorMode: {
          defaultMode: "dark",
          },
    navbar: {
      title: 'Hornet',
      logo: {
        alt: 'IOTA',
        src: 'img/logo/Logo_Swirl_Dark.png',
      },
      items: [
        {
          type: 'doc',
          docId: 'welcome',
          position: 'left',
          label: 'Documentation',
        },
//        {to: '/blog', label: 'Blog', position: 'left'},
        {
          href: 'https://github.com/iotaledger/hornet',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
      footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Welcome',
              to: '/welcome',
            },
            {
              label: 'Getting Started',
              to: '/getting_started/getting_started',
            },
            {
              label: 'Post Installation',
              to: '/post_installation/post_installation',
            },
            {
              label: 'API Reference',
              to: '/api_reference',
            },
            {
              label: 'Troubleshooting',
              to: '/troubleshooting',
            },
            {
              label: 'FAQ',
              to: '/faq',
            },
            {
              label: 'Contribute',
              to: '/contribute',
            },
            {
              label: 'Code of Conduct',
              to: '/code_of_conduct',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'Discord',
              href: 'https://discord.iota.org/',
            },
          ],
        },
        {
          title: 'Contribute',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/iotaledger/hornet',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} IOTA Foundation, Built with Docusaurus.`,
    },
    prism: {
        additionalLanguages: ['rust'],
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
    },
  },
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          routeBasePath:'/',
          // Please change this to your repo.
          editUrl:
            'https://github.com/iotaledger/hornet/tree/main/docs',
        },
        theme: {
          customCss: require.resolve('./src/css/iota.css'),
        },
      },
    ],
  ],
};
