
import { TopBar } from "../Components/TopBar";
import { useEffect } from "react";
import { importTorrentsList, selectTorrents } from "../slices/torrentSlice";
import { useAppDispatch, useAppSelector } from "../app/hooks";
import { useSelector } from "react-redux";
import TorrentsList from "../Components/TorrentsList";

export function TorrentsPage() {
    const torrentsFetchInterval = 100;
    const dispatch = useAppDispatch();
    const torrentsList = useAppSelector(selectTorrents)

    useEffect(() => {
        // Fetch torrents every 'torrentsFetchInterval' seconds
        const interval = setInterval(() => {
            dispatch(importTorrentsList());
        }, torrentsFetchInterval);

        return () => { clearInterval(interval); }
    }, []);

    return (
        <>
            <TopBar />
            <TorrentsList torrents={torrentsList} />
        </>
    )
}