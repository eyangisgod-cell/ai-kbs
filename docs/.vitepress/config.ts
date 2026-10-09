import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'AI-KBS',
  description: 'AI Knowledge Base System — open-source, AI-Agent-first',
  lang: 'en-US',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['meta', { name: 'theme-color', content: '#5b8def' }]
  ],

  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Getting Started', link: '/getting-started/' },
      { text: 'Architecture', link: '/architecture/' },
      { text: 'API', link: '/api/' },
      { text: 'CLI', link: '/cli/' },
      { text: 'MCP', link: '/mcp/' },
      { text: 'ADRs', link: '/adr/' }
    ],

    sidebar: {
      '/getting-started/': [
        {
          text: 'Getting Started',
          items: [
            { text: 'Overview', link: '/getting-started/' },
            { text: 'Installation', link: '/getting-started/installation' },
            { text: 'Quick Tour', link: '/getting-started/quick-tour' }
          ]
        }
      ],
      '/architecture/': [
        {
          text: 'Architecture',
          items: [
            { text: 'Overview', link: '/architecture/' },
            { text: 'Three-Layer Retrieval', link: '/architecture/retrieval' },
            { text: 'Workspace / Sync', link: '/architecture/workspace' }
          ]
        }
      ],
      '/api/': [
        {
          text: 'HTTP API',
          items: [
            { text: 'Overview', link: '/api/' },
            { text: 'Auth', link: '/api/auth' },
            { text: 'Documents', link: '/api/documents' },
            { text: 'Search', link: '/api/search' }
          ]
        }
      ],
      '/cli/': [
        {
          text: 'CLI',
          items: [
            { text: 'Overview', link: '/cli/' },
            { text: 'Install', link: '/cli/install' },
            { text: 'Commands', link: '/cli/commands' }
          ]
        }
      ],
      '/mcp/': [
        {
          text: 'MCP Server',
          items: [
            { text: 'Overview', link: '/mcp/' },
            { text: 'Install for AI Agents', link: '/mcp/install' },
            { text: 'Tools Reference', link: '/mcp/tools' }
          ]
        }
      ],
      '/adr/': [
        {
          text: 'Architecture Decision Records',
          items: [
            { text: 'Index', link: '/adr/' },
            { text: 'ADR-001 Fork WeKnora', link: '/adr/0001-fork-weknora' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/eyangisgod-cell/ai-kbs' }
    ],

    footer: {
      message: 'Released under the Apache 2.0 License.',
      copyright: 'Copyright © 2026 AI-KBS Authors'
    }
  }
})
