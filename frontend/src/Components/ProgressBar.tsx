import { torrentclient } from "../../wailsjs/go/models";

interface ProgressBarProps {
    barColorClass: string
    progressPercent: number;
}

export function ProgressBar({ progressPercent, barColorClass }: ProgressBarProps) {

    return (
        <div className="bg-gray-600 relative w-full" >
            <div className={barColorClass + " h-8"} style={{ width: progressPercent + "%" }}>
                <div className="absolute flex justify-center items-center w-full h-full">
                    {progressPercent == 100 ? progressPercent.toFixed(0) : progressPercent.toFixed(1)}%
                </div>
            </div>
        </div >
    )
}