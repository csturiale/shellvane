import { Component, inject, signal } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { TranslocoDirective } from '@jsverse/transloco';

import { writeClipboard } from '../core/clipboard';

/**
 * What the sign-in bookmarklet runs on the vault's web page, once signed in
 * in the user's browser: the values the page keeps in its storage and
 * cookies (one is the session; the app finds it by trying them on the
 * vault's API), sent to the app with a shellvane://signin link. The app
 * takes them only while it waits for this sign-in.
 */
export function signInBookmarklet(): string {
  const src =
    `(function(){var o={};[sessionStorage,localStorage].forEach(function(s,n){try{for(var i=0;i<s.length;i++){var k=s.key(i),v=s.getItem(k)||'';` +
    `if(v.length>=16&&v.length<8192)o[(n?'l:':'s:')+k]=v;}}catch(e){}});` +
    `document.cookie.split(';').forEach(function(c){var i=c.indexOf('=');var v=i>0?decodeURIComponent(c.slice(i+1).trim()):'';` +
    `if(v.length>=16&&v.length<8192)o['c:'+c.slice(0,i).trim()]=v;});` +
    `var k=Object.keys(o);if(!k.length){alert('Shellvane: sign in first, then click again.');return;}` +
    `location.href='shellvane://signin?'+k.map(function(x){return encodeURIComponent(x)+'='+encodeURIComponent(o[x]);}).join('&');})();`;
  return 'javascript:' + encodeURIComponent(src);
}

/** The sign-in bookmarklet as a link to drag to the bookmarks bar, or to copy. */
@Component({
  selector: 'app-signin-bookmarklet',
  imports: [TranslocoDirective],
  templateUrl: './signin-bookmarklet.html',
  styleUrl: './signin-bookmarklet.scss',
})
export class SignInBookmarklet {
  private readonly code = signInBookmarklet();
  protected readonly link = inject(DomSanitizer).bypassSecurityTrustUrl(this.code);
  protected readonly copied = signal(false);

  protected async copyCode(): Promise<void> {
    await writeClipboard(this.code);
    this.copied.set(true);
  }
}
