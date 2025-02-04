import { torrentclient } from "../../wailsjs/go/models"
import TorrentListItem from "./TorrentListItem"


interface TorrentsListProps {
    torrents: torrentclient.TorrentDTO[]
}

export default function TorrentsList({ torrents }: TorrentsListProps) {

    console.log(torrents)
    return (
        <div>
            <table className="table w-full">
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Size</th>
                        <th>Progress</th>
                        <th>Down Speed</th>
                        <th>Up Speed</th>
                        <th>Ratio</th>
                        <th>Private</th>
                    </tr>
                </thead>
                <tbody>
                    {torrents.map(t =>
                        <TorrentListItem key={t.torrentData.infohash} torrent={t}></TorrentListItem>
                    )}
                </tbody>
            </table>

        </div>
    )
}