import { Outlet } from 'react-router-dom';

export function Layout() {
    return (
        <div className="min-h-screen bg-gray-50 text-gray-900 font-sans">
            {/* <header className="bg-white border-b border-gray-200">
                <div className="max-w-5xl mx-auto px-8 h-16 flex items-center gap-2">
                    <div className="w-8 h-8 bg-black rounded-full flex items-center justify-center text-white font-bold">
                        S
                    </div>
                    <span className="font-bold text-lg tracking-tight">selfhosted</span>
                </div>
            </header> */}

            <main className="max-w-5xl mx-auto p-8">
                <Outlet />
            </main>
        </div>
    );
}


