/**
 * * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

module.exports = {
  docs: [
    {
      type: "category",
      label: "Hornet",
      collapsed: false,
      items: [
        {
          type: 'doc',
          id: 'welcome',
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
              id: 'how_tos/using_docker',
              label: 'Install Hornet using Docker',
            },
            {
              type: 'doc',
              id: 'how_tos/post_installation',
              label: 'Post Installation',
            },
            {
              type: 'doc',
              id: 'how_tos/run_as_a_verifier',
              label: 'Run a Node as a Verifier',
            }
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
            }
          ]
        }
      ]
    }
  ]
};
