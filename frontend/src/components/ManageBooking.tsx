import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { format, parseISO, addDays, startOfToday } from 'date-fns';
import { Calendar, Clock, User, Mail, Video, MessageCircle, ArrowRight, CheckCircle, XCircle, AlertCircle, RefreshCw } from 'lucide-react';

interface Booking {
  id: string;
  name: string;
  email: string;
  time: string;
  platform: string;
}

export default function ManageBooking({ id }: { id: string }) {
  const [booking, setBooking] = useState<Booking | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<'details' | 'cancel' | 'reschedule'>('details');
  const [actionStatus, setActionStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');

  // Reschedule state
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null);
  const [availableSlots, setAvailableSlots] = useState<string[]>([]);
  const [fetchingSlots, setFetchingSlots] = useState(false);

  const today = startOfToday();
  const nextDates = Array.from({ length: 7 }, (_, i) => addDays(today, i));

  useEffect(() => {
    fetchBooking();
  }, [id]);

  useEffect(() => {
    if (view === 'reschedule' && selectedDate) {
      setFetchingSlots(true);
      const dateStr = format(selectedDate, 'yyyy-MM-dd');
      fetch(`http://localhost:8080/api/availability?date=${dateStr}`)
        .then((res) => res.json())
        .then((data) => {
          setAvailableSlots(data.slots || []);
          setFetchingSlots(false);
        })
        .catch((err) => {
          console.error(err);
          setAvailableSlots(['10:00', '11:30', '14:00', '15:30']); // Fallback
          setFetchingSlots(false);
        });
    }
  }, [selectedDate, view]);

  const fetchBooking = async () => {
    try {
      setLoading(true);
      const response = await fetch(`http://localhost:8080/api/booking/${id}`);
      if (!response.ok) {
        throw new Error('Booking not found');
      }
      const data = await response.json();
      setBooking(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load booking');
    } finally {
      setLoading(false);
    }
  };

  const handleCancel = async () => {
    try {
      setActionStatus('loading');
      const response = await fetch(`http://localhost:8080/api/booking/${id}`, {
        method: 'DELETE',
      });
      if (response.ok) {
        setActionStatus('success');
      } else {
        throw new Error('Failed to cancel');
      }
    } catch (err) {
      setActionStatus('error');
    }
  };

  const handleReschedule = async () => {
    if (!selectedDate || !selectedSlot) return;

    try {
      setActionStatus('loading');
      const timeStr = `${format(selectedDate, 'yyyy-MM-dd')}T${selectedSlot}:00Z`;
      const response = await fetch(`http://localhost:8080/api/booking/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ time: timeStr }),
      });

      if (response.ok) {
        setActionStatus('success');
      } else {
        throw new Error('Failed to reschedule');
      }
    } catch (err) {
      setActionStatus('error');
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
      </div>
    );
  }

  if (error || !booking) {
    return (
      <div className="text-center bg-gray-900 rounded-2xl shadow-xl border border-gray-800 p-8 max-w-md mx-auto">
        <AlertCircle className="mx-auto text-red-500 mb-4" size={48} />
        <h2 className="text-xl font-bold text-white mb-2">Booking Not Found</h2>
        <p className="text-gray-400">The booking link may be invalid or the meeting was already cancelled.</p>
        <button
          onClick={() => window.location.href = '/'}
          className="mt-6 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
        >
          Book a New Meeting
        </button>
      </div>
    );
  }

  const bookingDate = parseISO(booking.time);

  return (
    <div className="w-full max-w-2xl mx-auto">
      <div className="bg-gray-900 rounded-2xl shadow-xl border border-gray-800 overflow-hidden">

        {/* Header */}
        <div className="p-6 border-b border-gray-800 bg-gray-800/30">
          <h2 className="text-xl font-semibold text-white flex items-center gap-2">
            <Calendar className="text-blue-400" />
            Manage Your Booking
          </h2>
          <p className="text-gray-400 text-sm mt-1">View, reschedule, or cancel your upcoming chat.</p>
        </div>

        <div className="p-6">
          <AnimatePresence mode="wait">

            {/* View Details */}
            {view === 'details' && (
              <motion.div
                key="details"
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                className="space-y-6"
              >
                <div className="flex items-start gap-4 p-4 rounded-xl bg-gray-800/50 border border-gray-700">
                  <div className="w-12 h-12 bg-blue-500/20 text-blue-400 rounded-full flex items-center justify-center shrink-0">
                    <CheckCircle size={24} />
                  </div>
                  <div>
                    <h3 className="text-lg font-medium text-white mb-1">Confirmed Meeting</h3>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-y-3 gap-x-6 mt-3 text-sm">
                      <div className="flex items-center gap-2 text-gray-300">
                        <User size={16} className="text-gray-500" /> {booking.name}
                      </div>
                      <div className="flex items-center gap-2 text-gray-300">
                        <Mail size={16} className="text-gray-500" /> {booking.email}
                      </div>
                      <div className="flex items-center gap-2 text-gray-300">
                        <Calendar size={16} className="text-gray-500" /> {format(bookingDate, 'MMMM d, yyyy')}
                      </div>
                      <div className="flex items-center gap-2 text-gray-300">
                        <Clock size={16} className="text-gray-500" /> {format(bookingDate, 'HH:mm')} (30 min)
                      </div>
                      <div className="flex items-center gap-2 text-gray-300 sm:col-span-2">
                        {booking.platform === 'Google Meet' ? <Video size={16} className="text-gray-500"/> : <MessageCircle size={16} className="text-gray-500"/>}
                        {booking.platform}
                      </div>
                    </div>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <button
                    onClick={() => setView('reschedule')}
                    className="flex items-center justify-center gap-2 py-3 px-4 rounded-xl border border-gray-700 bg-gray-800 hover:bg-gray-700 hover:text-white transition-all text-gray-300 font-medium"
                  >
                    <RefreshCw size={18} /> Reschedule
                  </button>
                  <button
                    onClick={() => setView('cancel')}
                    className="flex items-center justify-center gap-2 py-3 px-4 rounded-xl border border-red-900/30 bg-red-900/10 hover:bg-red-900/20 text-red-400 transition-all font-medium"
                  >
                    <XCircle size={18} /> Cancel Meeting
                  </button>
                </div>
              </motion.div>
            )}

            {/* Cancel View */}
            {view === 'cancel' && (
              <motion.div
                key="cancel"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
              >
                {actionStatus === 'success' ? (
                  <div className="text-center py-6">
                    <XCircle className="mx-auto text-red-500 mb-4" size={48} />
                    <h3 className="text-xl font-bold text-white mb-2">Meeting Cancelled</h3>
                    <p className="text-gray-400 mb-6">Your booking has been successfully cancelled.</p>
                    <a href="/" className="px-6 py-2 bg-gray-800 hover:bg-gray-700 text-white rounded-lg transition-colors">Return to Home</a>
                  </div>
                ) : (
                  <div className="space-y-6">
                    <div className="p-4 rounded-xl bg-red-900/10 border border-red-900/30 text-center">
                      <h3 className="text-lg font-medium text-white mb-2">Are you sure?</h3>
                      <p className="text-gray-400 text-sm">
                        You are about to cancel your meeting on {format(bookingDate, 'MMMM d')} at {format(bookingDate, 'HH:mm')}.
                        This action cannot be undone.
                      </p>
                    </div>

                    {actionStatus === 'error' && (
                      <div className="text-red-400 text-sm text-center">Failed to cancel meeting. Please try again.</div>
                    )}

                    <div className="flex gap-4">
                      <button
                        onClick={() => setView('details')}
                        className="flex-1 py-3 px-4 bg-gray-800 hover:bg-gray-700 text-white rounded-lg transition-colors"
                      >
                        Keep Meeting
                      </button>
                      <button
                        onClick={handleCancel}
                        disabled={actionStatus === 'loading'}
                        className="flex-1 py-3 px-4 bg-red-600 hover:bg-red-700 text-white font-medium rounded-lg flex items-center justify-center gap-2 transition-all disabled:opacity-70"
                      >
                        {actionStatus === 'loading' ? 'Cancelling...' : 'Yes, Cancel'}
                      </button>
                    </div>
                  </div>
                )}
              </motion.div>
            )}

            {/* Reschedule View */}
            {view === 'reschedule' && (
              <motion.div
                key="reschedule"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
              >
                {actionStatus === 'success' ? (
                  <div className="text-center py-6">
                    <CheckCircle className="mx-auto text-green-500 mb-4" size={48} />
                    <h3 className="text-xl font-bold text-white mb-2">Meeting Rescheduled!</h3>
                    <p className="text-gray-400 mb-6">
                      Your meeting is now set for {selectedDate && format(selectedDate, 'MMMM d')} at {selectedSlot}.
                    </p>
                    <button
                      onClick={() => {
                        fetchBooking();
                        setView('details');
                        setActionStatus('idle');
                        setSelectedDate(null);
                        setSelectedSlot(null);
                      }}
                      className="px-6 py-2 bg-gray-800 hover:bg-gray-700 text-white rounded-lg transition-colors"
                    >
                      View Details
                    </button>
                  </div>
                ) : (
                  <div>
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-lg font-medium text-white">Select New Time</h3>
                      <button
                        onClick={() => setView('details')}
                        className="text-sm text-gray-400 hover:text-white"
                      >
                        Cancel Reschedule
                      </button>
                    </div>

                    {!selectedDate ? (
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                        {nextDates.map((date) => (
                          <button
                            key={date.toISOString()}
                            onClick={() => setSelectedDate(date)}
                            className="flex flex-col items-center justify-center p-4 rounded-xl border border-gray-800 bg-gray-800/50 hover:bg-blue-600/20 hover:border-blue-500/50 transition-all group"
                          >
                            <span className="text-sm text-gray-400 group-hover:text-blue-300">
                              {format(date, 'EEE')}
                            </span>
                            <span className="text-2xl font-bold text-white mt-1">
                              {format(date, 'd')}
                            </span>
                            <span className="text-xs text-gray-500 group-hover:text-blue-400 mt-1">
                              {format(date, 'MMM')}
                            </span>
                          </button>
                        ))}
                      </div>
                    ) : !selectedSlot ? (
                      <div>
                        <div className="flex items-center justify-between mb-4">
                          <p className="text-sm text-gray-400">Times for {format(selectedDate, 'MMM d')}</p>
                          <button onClick={() => setSelectedDate(null)} className="text-sm text-blue-400 hover:text-blue-300">Change Date</button>
                        </div>

                        {fetchingSlots ? (
                          <div className="flex justify-center py-8">
                            <div className="animate-spin rounded-full h-6 w-6 border-t-2 border-b-2 border-blue-500"></div>
                          </div>
                        ) : availableSlots.length > 0 ? (
                          <div className="grid grid-cols-3 gap-3">
                            {availableSlots.map((slot) => (
                              <button
                                key={slot}
                                onClick={() => setSelectedSlot(slot)}
                                className="py-3 px-4 rounded-xl border border-gray-800 bg-gray-800/50 hover:bg-blue-600 hover:border-blue-500 hover:text-white transition-all text-gray-300 font-medium"
                              >
                                {slot}
                              </button>
                            ))}
                          </div>
                        ) : (
                          <div className="text-center py-8 text-gray-400">No available slots.</div>
                        )}
                      </div>
                    ) : (
                      <div className="space-y-4">
                         <div className="p-4 rounded-xl bg-blue-900/10 border border-blue-900/30">
                           <p className="text-gray-300 text-sm">Reschedule to:</p>
                           <p className="text-lg font-medium text-white mt-1">
                             {format(selectedDate, 'MMMM d, yyyy')} at {selectedSlot}
                           </p>
                         </div>
                         <div className="flex gap-4">
                           <button
                             onClick={() => setSelectedSlot(null)}
                             className="flex-1 py-3 px-4 bg-gray-800 hover:bg-gray-700 text-white rounded-lg transition-colors"
                           >
                             Change Time
                           </button>
                           <button
                             onClick={handleReschedule}
                             disabled={actionStatus === 'loading'}
                             className="flex-1 py-3 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg shadow-lg shadow-blue-500/30 flex items-center justify-center gap-2 transition-all disabled:opacity-70"
                           >
                             {actionStatus === 'loading' ? 'Updating...' : 'Confirm'}
                           </button>
                         </div>
                         {actionStatus === 'error' && (
                           <div className="text-red-400 text-sm text-center">Failed to reschedule. Please try again.</div>
                         )}
                      </div>
                    )}
                  </div>
                )}
              </motion.div>
            )}

          </AnimatePresence>
        </div>
      </div>
    </div>
  );
}
