# Markdown Site Skill

Deploy a minimal markdown site using Docsify. Zero build step, client-side rendering.

## Structure

```
/site
  index.html      # Docsify loader
  README.md       # Home page content
  _sidebar.md     # Navigation (optional)
  page-name.md    # Additional pages
  Dockerfile      # nginx static server
```

## index.html

```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Site Title</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/docsify@4/themes/vue.css">
</head>
<body>
  <div id="app"></div>
  <script>
    window.$docsify = {
      name: 'Site Title',
      loadSidebar: true,
      subMaxLevel: 2,
      auto2top: true
    }
  </script>
  <script src="https://cdn.jsdelivr.net/npm/docsify@4"></script>
</body>
</html>
```

## _sidebar.md (optional navigation)

```markdown
- [Home](/)
- [About](about.md)
- [Guide](guide.md)
```

## Dockerfile

```dockerfile
FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
```

## Themes

Change the CSS link for different looks:
- `themes/vue.css` - clean, default
- `themes/dark.css` - dark mode
- `themes/pure.css` - minimal
- `themes/dolphin.css` - blue tint

## Database

Use SQLite if the site needs any data persistence. For static content, no database needed.

## Deployment

1. Write markdown content in .md files
2. Create index.html with Docsify config
3. Create Dockerfile with nginx
4. Deploy with deploy_app tool
