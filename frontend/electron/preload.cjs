const { contextBridge } = require('electron');

function readArg(prefix) {
  const arg = process.argv.find((value) => value.startsWith(prefix));
  return arg ? arg.slice(prefix.length) : '';
}

const apiToken = readArg('--plainshelf-api-token=');
const tokenHeader = readArg('--plainshelf-token-header=') || 'X-PlainShelf-Token';
const baseURL = readArg('--plainshelf-base-url=');
const profileDir = readArg('--plainshelf-profile-dir=');

contextBridge.exposeInMainWorld('plainshelf', {
  getApiToken: () => apiToken,
  getApiTokenHeader: () => tokenHeader,
  getApiBaseURL: () => baseURL,
  getProfileDir: () => profileDir
});
