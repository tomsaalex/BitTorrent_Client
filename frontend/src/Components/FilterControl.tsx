import { useId } from "react";
import { FaSearch } from "react-icons/fa";

export function FilterControl() {
    const torrentNameInputId = useId()

    return (
        <>
            <div className="flex flex-row items-center">
                <label className="mx-2 align-middle" htmlFor={torrentNameInputId}>Filter by torrent name:</label>
                <FaSearch className="ml-2 align-middle" size="1em" />
                <input name="Torrent filter input" id={torrentNameInputId} className="mx-2 h-1/2"></input>
            </div>
        </>
    );
}