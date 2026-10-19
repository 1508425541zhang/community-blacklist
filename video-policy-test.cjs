const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const path=require('node:path');
const context={URL,URLSearchParams};vm.createContext(context);
vm.runInContext(fs.readFileSync(path.join(__dirname,'web/video-policy.js'),'utf8'),context);
const policy=context.VideoPolicy,cases=JSON.parse(fs.readFileSync(path.join(__dirname,'video-policy-cases.json'),'utf8'));
for(const test of cases){if(test.canonical)assert.equal(policy.canonical(test.url).url,test.canonical,test.url);else assert.throws(()=>policy.canonical(test.url),undefined,test.url)}
assert.equal(policy.normalize('SteamID: 76561190000000001 联系人 test@example.com'),'SteamID: 76561190000000001 联系人 test@example.com');
assert.throws(()=>policy.prepare({evidence:'https://youtu.be/dQw4w9WgXcQ https://evil.com/file'}));
assert.equal(policy.prepare({password:'https://anything.example/private'}).password,'https://anything.example/private');
console.log(`${cases.length} shared video URL cases and text validation passed.`);
