export type QbitTorrent = {
  hash: string;
  name: string;
  size: number;
  progress: number;
  state: string;
  seeders: number;
  leechers: number;
  downloaded: number;
  uploaded: number;
  dlspeed: number;
  upspeed: number;
  eta: number;
};

export type HardlinkArtifact = {
  name: string;
  size: number;
  linked: boolean;
};

export type HardlinkProgress = {
  linked: number;
  total: number;
  complete: boolean;
  done: HardlinkArtifact[];
  remaining: HardlinkArtifact[];
};

export type RequestTorrentInfo = {
  request_status: string;
  torrent_name: string;
  indexer: string;
  quality: string;
  torrent_hash?: string;
  torrent?: QbitTorrent;
  hardlink?: HardlinkProgress;
  message?: string;
};
