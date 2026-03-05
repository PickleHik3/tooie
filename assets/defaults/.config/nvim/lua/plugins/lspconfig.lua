-- LSP configuration moved to termux.lua for Termux environment
-- All servers point to local installations instead of Mason
return {
  "neovim/nvim-lspconfig",
  enabled = false,  -- Disabled - use termux.lua instead
  opts = {
    ---@type lspconfig.options
    servers = {
      -- Lua
      lua_ls = {
        settings = {
          Lua = {
            workspace = {
              checkThirdParty = false,
            },
            completion = {
              callSnippet = "Replace",
            },
          },
        },
      },

      -- Python
      pyright = {
        settings = {
          python = {
            analysis = {
              extraPaths = {},
              typeCheckingMode = "standard",
            },
          },
        },
      },

      -- TypeScript/JavaScript
      ts_ls = {
        cmd = { "typescript-language-server", "--stdio" },
        init_options = {
          preferences = {
            disableSuggestions = false,
          },
        },
      },

      -- Go
      gopls = {
        settings = {
          gopls = {
            gofumpt = true,
            staticcheck = true,
            usePlaceholders = true,
          },
        },
      },

      -- Rust
      rust_analyzer = {
        settings = {
          ["rust-analyzer"] = {
            checkOnSave = {
              command = "clippy",
            },
            cargo = {
              loadOutDirsFromCheck = true,
            },
            procMacro = {
              enable = true,
            },
          },
        },
      },

      -- C/C++
      clangd = {
        settings = {
          clangd = {
            arguments = {
              "--background-index",
              "--clang-tidy",
              "--header-insertion=iwyu",
            },
          },
        },
      },

      -- HTML
      html = {},

      -- CSS
      cssls = {},

      -- JSON
      jsonls = {},

      -- YAML
      yamlls = {
        settings = {
          yaml = {
            schemaStore = {
              enable = true,
              url = "https://www.schemastore.org/json/",
            },
          },
        },
      },

      -- Bash
      bashls = {},

      -- Markdown
      marksman = {},

      -- TOML
      taplo = {},

      -- Assembly
      asm_lsp = {},
    },

    -- Server setup customization
    ---@type table<string, fun(server:string, opts:_.lspconfig.options):boolean?>
    setup = {
      -- Example: custom rust-analyzer setup
      -- rust_analyzer = function(_, opts)
      --   return true
      -- end,
    },
  },

  -- Add treesitter support for better syntax awareness
  {
    "nvim-treesitter/nvim-treesitter",
    opts = {
      ensure_installed = {
        "bash",
        "c",
        "cmake",
        "cpp",
        "css",
        "go",
        "html",
        "javascript",
        "json",
        "lua",
        "markdown",
        "markdown_inline",
        "python",
        "rust",
        "toml",
        "typescript",
        "vim",
        "yaml",
      },
    },
  },
}
