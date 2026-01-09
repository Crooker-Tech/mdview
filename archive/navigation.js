// mdview archive navigation system
// Handles loading embedded pages and overlay management

(function() {
  'use strict';

  // Navigation state
  window.mdviewHistory = [];
  window.mdviewCurrentPage = window.mdviewArchive ? window.mdviewArchive.root : null;

  // Path resolution utilities
  function normalizePath(path) {
    // Convert backslashes to forward slashes
    return path.replace(/\\/g, '/');
  }

  function resolvePath(basePath, relativePath) {
    // Remove query strings and fragments
    relativePath = relativePath.split('?')[0].split('#')[0];

    // If relative path is absolute, return it
    if (relativePath.startsWith('/') || /^[a-zA-Z]:/.test(relativePath)) {
      return normalizePath(relativePath);
    }

    // Get directory of base path
    const baseDir = basePath.substring(0, basePath.lastIndexOf('/'));

    // Split relative path into parts
    const parts = relativePath.split('/');
    const baseParts = baseDir.split('/').filter(p => p);

    for (let i = 0; i < parts.length; i++) {
      if (parts[i] === '..') {
        baseParts.pop();
      } else if (parts[i] !== '.' && parts[i] !== '') {
        baseParts.push(parts[i]);
      }
    }

    return baseParts.join('/');
  }

  function getRelativePath(absolutePath) {
    if (!window.mdviewArchive || !window.mdviewArchive.root) {
      return absolutePath;
    }

    const root = normalizePath(window.mdviewArchive.root);
    const normalized = normalizePath(absolutePath);
    const rootDir = root.substring(0, root.lastIndexOf('/'));

    // If path starts with root directory, make it relative
    if (normalized.startsWith(rootDir + '/')) {
      return normalized.substring(rootDir.length + 1);
    }

    // Otherwise, try to find common prefix
    const rootParts = rootDir.split('/').filter(p => p);
    const pathParts = normalized.split('/').filter(p => p);

    // Find common prefix length
    let commonLength = 0;
    while (commonLength < rootParts.length &&
           commonLength < pathParts.length &&
           rootParts[commonLength] === pathParts[commonLength]) {
      commonLength++;
    }

    // Build relative path
    const upCount = rootParts.length - commonLength;
    const upPath = new Array(upCount).fill('..').join('/');
    const downPath = pathParts.slice(commonLength).join('/');

    if (upPath && downPath) {
      return upPath + '/' + downPath;
    } else if (upPath) {
      return upPath;
    } else {
      return downPath;
    }
  }

  // Decompression function
  function decompressPage(compressed) {
    try {
      // Decode base64
      const decoded = atob(compressed);

      // Convert to Uint8Array
      const uint8Array = new Uint8Array(decoded.length);
      for (let i = 0; i < decoded.length; i++) {
        uint8Array[i] = decoded.charCodeAt(i);
      }

      // Decompress with pako
      const decompressed = pako.inflate(uint8Array, { to: 'string' });
      return decompressed;
    } catch (e) {
      console.error('Failed to decompress page:', e);
      return null;
    }
  }

  // Extract article content from full HTML
  function extractArticle(html) {
    const startTag = '<article class="markdown-body">';
    const endTag = '</article>';

    const startIdx = html.indexOf(startTag);
    if (startIdx === -1) {
      // Fallback: return everything
      return html;
    }

    const endIdx = html.indexOf(endTag, startIdx);
    if (endIdx === -1) {
      return html;
    }

    // Return content including tags
    return html.substring(startIdx, endIdx + endTag.length);
  }

  // Show overlay with page content
  function showOverlay(articleHTML) {
    const overlay = document.getElementById('mdview-overlay');
    const body = document.getElementById('mdview-overlay-body');

    if (!overlay || !body) {
      console.error('Overlay elements not found');
      return;
    }

    body.innerHTML = articleHTML;
    overlay.classList.add('visible');

    // Re-initialize syntax highlighting if available
    if (window.hljs) {
      body.querySelectorAll('pre code').forEach((block) => {
        hljs.highlightBlock(block);
      });
    }

    // Scroll to top
    overlay.scrollTop = 0;
  }

  // Hide overlay
  function hideOverlay() {
    const overlay = document.getElementById('mdview-overlay');
    if (overlay) {
      overlay.classList.remove('visible');
    }

    // Restore previous page
    if (window.mdviewHistory.length > 0) {
      window.mdviewCurrentPage = window.mdviewHistory[window.mdviewHistory.length - 1];
    }
  }

  // Load embedded page
  function loadEmbeddedPage(href) {
    if (!window.mdviewArchive || !window.mdviewArchive.pages) {
      console.error('Archive data not available');
      return false;
    }

    // Resolve absolute path
    const absolutePath = resolvePath(window.mdviewCurrentPage, href);

    // Get relative path for lookup
    const relativePath = getRelativePath(absolutePath);

    // Look up in archive
    const compressed = window.mdviewArchive.pages[relativePath];
    if (!compressed) {
      console.warn('Page not found in archive:', relativePath);
      return false;
    }

    // Decompress
    const html = decompressPage(compressed);
    if (!html) {
      console.error('Failed to decompress page:', relativePath);
      return false;
    }

    // Extract article content
    const articleHTML = extractArticle(html);

    // Check if this is a back link (links to previous page in history)
    const isBackLink = window.mdviewHistory.length > 0 &&
                       absolutePath === window.mdviewHistory[window.mdviewHistory.length - 1];

    if (isBackLink) {
      // Hide overlay and go back
      hideOverlay();
      window.mdviewHistory.pop();
    } else {
      // Save current page to history
      window.mdviewHistory.push(window.mdviewCurrentPage);

      // Update current page
      window.mdviewCurrentPage = absolutePath;

      // Show in overlay
      showOverlay(articleHTML);
    }

    return true;
  }

  // Check if link is local .md file
  function isLocalMarkdownLink(href) {
    // Skip empty, anchors, protocols
    if (!href ||
        href.startsWith('#') ||
        href.startsWith('mailto:') ||
        href.startsWith('tel:') ||
        href.startsWith('http://') ||
        href.startsWith('https://') ||
        href.includes('://')) {
      return false;
    }

    // Extract path without fragment/query
    const path = href.split('#')[0].split('?')[0];

    // Check if ends with .md
    return path.toLowerCase().endsWith('.md');
  }

  // Click handler
  function handleClick(e) {
    const link = e.target.closest('a');
    if (!link) return;

    const href = link.getAttribute('href');
    if (!href) return;

    // Check if it's a local markdown link
    if (isLocalMarkdownLink(href)) {
      e.preventDefault();
      loadEmbeddedPage(href);
    }
  }

  // Close button handler
  function handleClose(e) {
    if (e.target.id === 'mdview-overlay' ||
        e.target.classList.contains('mdview-close-btn')) {
      e.preventDefault();
      hideOverlay();

      if (window.mdviewHistory.length > 0) {
        window.mdviewHistory.pop();
      }
    }
  }

  // Initialize
  function init() {
    // Add click listener to document
    document.addEventListener('click', handleClick);

    // Add close listener to overlay
    const overlay = document.getElementById('mdview-overlay');
    if (overlay) {
      overlay.addEventListener('click', handleClose);

      // Prevent clicks inside overlay content from closing
      const content = overlay.querySelector('.mdview-overlay-content');
      if (content) {
        content.addEventListener('click', function(e) {
          e.stopPropagation();
        });
      }
    }

    // Log archive info for debugging
    if (window.mdviewArchive) {
      const pageCount = Object.keys(window.mdviewArchive.pages).length;
      console.log('mdview archive loaded with', pageCount, 'pages');
    }
  }

  // Run init when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
