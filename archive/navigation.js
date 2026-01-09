// mdview archive navigation system
// Replaces article content when navigating between embedded pages

(function() {
  'use strict';

  // State
  var originalContent = null;  // Saved original page content
  var currentPage = null;      // Current embedded page key (null = original)

  // Decompression function
  function decompressPage(compressed) {
    try {
      var decoded = atob(compressed);
      var uint8Array = new Uint8Array(decoded.length);
      for (var i = 0; i < decoded.length; i++) {
        uint8Array[i] = decoded.charCodeAt(i);
      }
      return pako.inflate(uint8Array, { to: 'string' });
    } catch (e) {
      console.error('Failed to decompress page:', e);
      return null;
    }
  }

  // Extract article innerHTML from full HTML
  function extractArticleContent(html) {
    var startTag = '<article class="markdown-body">';
    var endTag = '</article>';

    var startIdx = html.indexOf(startTag);
    if (startIdx === -1) return html;

    var contentStart = startIdx + startTag.length;
    var endIdx = html.indexOf(endTag, contentStart);
    if (endIdx === -1) return html;

    return html.substring(contentStart, endIdx);
  }

  // Get the main article element
  function getArticle() {
    return document.querySelector('article.markdown-body');
  }

  // Global function to load a page from the archive
  window.mdviewLoadPage = function(archiveKey) {
    if (!window.mdviewArchive || !window.mdviewArchive.pages) {
      console.error('Archive data not available');
      return;
    }

    var article = getArticle();
    if (!article) {
      console.error('Article element not found');
      return;
    }

    // Save original content on first navigation
    if (originalContent === null) {
      originalContent = article.innerHTML;
    }

    // Look up in archive
    var compressed = window.mdviewArchive.pages[archiveKey];
    if (!compressed) {
      console.warn('Page not found in archive:', archiveKey);
      return;
    }

    // Decompress
    var html = decompressPage(compressed);
    if (!html) {
      console.error('Failed to decompress page:', archiveKey);
      return;
    }

    // Extract and replace content
    var content = extractArticleContent(html);
    article.innerHTML = content;
    currentPage = archiveKey;

    // Re-initialize syntax highlighting if available
    if (window.hljs) {
      article.querySelectorAll('pre code').forEach(function(block) {
        hljs.highlightBlock(block);
      });
    }

    // Scroll to top
    window.scrollTo(0, 0);
  };

  // Global function to return to original page
  window.mdviewLoadOriginal = function() {
    if (originalContent === null) return;

    var article = getArticle();
    if (!article) return;

    article.innerHTML = originalContent;
    currentPage = null;

    // Re-initialize syntax highlighting if available
    if (window.hljs) {
      article.querySelectorAll('pre code').forEach(function(block) {
        hljs.highlightBlock(block);
      });
    }

    // Scroll to top
    window.scrollTo(0, 0);
  };

  // Initialize
  function init() {
    if (window.mdviewArchive) {
      var pageCount = Object.keys(window.mdviewArchive.pages).length;
      console.log('mdview archive loaded with', pageCount, 'pages');
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
