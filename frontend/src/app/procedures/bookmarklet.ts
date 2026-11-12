import { htmlToMarkdown, pageContent } from './html-to-markdown';

/** Starts the Markdown the bookmarklet sends (core/procedures.Marker). */
export const MARKER = '<!-- shellvane-procedure -->';

/** Longest Markdown (base64) sent in the link itself; longer pages go through the clipboard. */
const MAX_INLINE = 6000;

/**
 * The "Send to Shellvane" bookmarklet: run on a page already open (and
 * signed in) in the browser, it turns the selection or the page's article
 * into Markdown and opens shellvane://procedure with it. A long page is put
 * on the clipboard, which Shellvane reads only because it starts with the
 * marker.
 */
export function bookmarklet(): string {
  const src =
    `(function(){var h2m=${htmlToMarkdown.toString()};var main=${pageContent.toString()};` +
    `var md=${JSON.stringify(MARKER)}+'\\n'+h2m(main(document));` +
    `var q='title='+encodeURIComponent(document.title)+'&url='+encodeURIComponent(location.href);` +
    `var b=btoa(unescape(encodeURIComponent(md))).replace(/\\+/g,'-').replace(/\\//g,'_').replace(/=+$/,'');` +
    `if(b.length<${MAX_INLINE}){location.href='shellvane://procedure?'+q+'&md='+b;}` +
    `else{navigator.clipboard.writeText(md).then(function(){location.href='shellvane://procedure?'+q+'&clip=1';},function(e){alert('Shellvane: '+e);});}})();`;
  return 'javascript:' + encodeURIComponent(src);
}
