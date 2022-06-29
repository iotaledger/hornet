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
      id: 'getting_started/welcome',
    },
    {
      type: 'doc',
      id: 'getting_started/getting_started',
    },
    {
      type: 'category',
      label: 'How to',
      items: [
        {
          type: 'doc',
          id: 'how_tos/hornet_apt_repository',
          label: 'Install Hornet via apt repository',
        },
        {
          type: 'doc',
          id: 'how_tos/using_docker',
          label: 'Install Hornet using Docker',
        },
        {
          type: 'doc',
          id: 'how_tos/using_docker_compose',
          label: 'Install Hornet using Docker Compose',
        }, 
        {
          type: 'doc',
          id: 'how_tos/bootstrap_from_a_genesis_snapshot',
          label: 'Bootstrap from a genesis snapshot',
        }, 
        {
          type: 'doc',
          id: 'how_tos/private_tangle',
          label: 'Run a private tangle',
        },
        {
          type: 'doc',
          id: 'how_tos/post_installation',
          label: 'Post Installation',
        }, 
        {
          type: 'doc',
          id: 'how_tos/managing_a_node',
          label: 'Manage a Node',
        }, 
        {
          type: 'doc',
          id: 'how_tos/run_as_a_verifier',
          label: 'Run a Node as a Verifier',
        },
      ]
    },
    {
      type: 'category',
      label: 'Key Concepts',
      items: [
        {
          type: 'doc',
          id: 'explanations/nodes_101',
          label: 'Nodes 101',
        }, 
        {
          type: 'doc',
          id: 'explanations/security_101',
          label: 'Security 101',
        },
        {
          type: 'doc',
          id: 'explanations/peering',
          label: 'Peering',
        }, 
      ]
    },
    {
      type: 'category',
      label: 'References',
      items: [
        {
          type: 'doc',
          id: 'references/configuration',
          label: 'Configuration',
        },
        {
          type: 'doc',
          id: 'references/api_reference',
          label: 'API Reference',
        },
        {
          type: 'doc',
          id: 'references/faq',
          label: 'FAQ',
        },
      ]
    },
    {
      type: 'doc',
      id: 'troubleshooting',
      label: 'Troubleshooting',
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