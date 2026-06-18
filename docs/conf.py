project = "ichsm"
copyright = "2026, Martin Hunt"
author = "Martin Hunt"

extensions = [
    "myst_parser",
]

source_suffix = {
    ".md": "markdown",
}

master_doc = "index"

exclude_patterns = [
    "_build",
    "Thumbs.db",
    ".DS_Store",
]

html_theme = "furo"
html_title = "ichsm"
html_theme_options = {
    "source_repository": "https://github.com/martinghunt/ichsm/",
    "source_branch": "main",
    "source_directory": "docs/",
}
html_static_path = [
    "_static",
]
