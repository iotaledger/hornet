/**
 * * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

module.exports = {
  mySidebar: [{
      type: 'doc',
      id: 'welcome',
    },
    {
      type: 'category',
      label: 'Getting Started',
      items: [{
        type: 'doc',
        id: 'getting_started/getting_started',
        label: 'Getting Started',
      }, {
        type: 'doc',
        id: 'getting_started/nodes_101',
        label: 'Nodes 101',
      }, {
        type: 'doc',
        id: 'getting_started/security_101',
        label: 'Security 101',
      }, {
        type: 'doc',
        id: 'getting_started/hornet_apt_repository',
        label: 'Hornet apt Repository',
      }, {
        type: 'doc',
        id: 'getting_started/using_docker',
        label: 'Using Docker',
      }, {
        type: 'doc',
        id: 'getting_started/using_docker_compose',
        label: 'Using Docker Compose',
      }, {
        type: 'doc',
        id: 'getting_started/bootstrap_from_a_genesis_snapshot',
        label: 'Bootstrapping From a Genesis Snapshot',
      }, {
        type: 'doc',
        id: 'getting_started/private_tangle',
        label: 'Private Tangle',
      }, ]
    },
    {
      type: 'category',
      label: 'Post Installation',
      items: [{
        type: 'doc',
        id: 'post_installation/post_installation',
        label: 'Post Installation',
      }, {
        type: 'doc',
        id: 'post_installation/managing_a_node',
        label: 'Managing a Node',
      }, {
        type: 'doc',
        id: 'post_installation/configuration',
        label: 'Configuration',
      }, {
        type: 'doc',
        id: 'post_installation/peering',
        label: 'Peering',
      }, {
        type: 'doc',
        id: 'post_installation/run_as_a_verifier',
        label: 'Run as a Verifier',
      },]
    },
    {
      type: 'doc',
      id: 'api_reference',
      label: 'API Reference',
    },
    {
      type: 'doc',
      id: 'troubleshooting',
      label: 'Troubleshooting',
    },
    {
      type: 'doc',
      id: 'faq',
      label: 'FAQ',
    },
    {
      type: 'doc',
      id: 'contribute',
      label: 'Contribute',
    },
    {
      type: 'doc',
      id: 'code_of_conduct',
      label: 'Code of Conduct',
    }
  ]
};