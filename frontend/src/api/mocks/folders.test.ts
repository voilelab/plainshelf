import { expect, it } from 'vitest';
import {
  addMockFolder,
  getMockFolderMeta,
  renameMockFolder,
  setMockFolderIcon,
  setMockFolderSortKey
} from './folders';

it('keeps icon and order metadata together through edits and renames', () => {
  addMockFolder('sort-test');
  setMockFolderIcon('sort-test', 'moon', '');
  const version = getMockFolderMeta()['sort-test']!.version;
  setMockFolderSortKey('sort-test', 0, version);
  expect(getMockFolderMeta()['sort-test']).toMatchObject({icon: 'moon', sort_key: 0});
  expect(() => setMockFolderSortKey('sort-test', 1, version)).toThrow();
  setMockFolderIcon('sort-test', '', getMockFolderMeta()['sort-test']!.version);
  renameMockFolder('sort-test', 'sort-renamed');
  expect(getMockFolderMeta()['sort-renamed']).toMatchObject({icon: '', sort_key: 0});
});
