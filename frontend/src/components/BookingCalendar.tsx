import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { format, addDays, startOfToday } from 'date-fns';
import { Calendar, Clock, User, Mail, Video, MessageCircle, ArrowRight, Instagram, Phone } from 'lucide-react';

export default function BookingCalendar() {
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null);
  const [availableSlots, setAvailableSlots] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [bookingStatus, setBookingStatus] = useState<'idle' | 'submitting' | 'success' | 'error'>('idle');

  const [formData, setFormData] = useState({
    name: '',
    email: '',
    platform: 'Google Meet',
  });

  const today = startOfToday();
  const nextDates = Array.from({ length: 7 }, (_, i) => addDays(today, i));

  useEffect(() => {
    if (selectedDate) {
      setLoading(true);
      const dateStr = format(selectedDate, 'yyyy-MM-dd');
      fetch(`http://localhost:8080/api/availability?date=${dateStr}`)
        .then((res) => res.json())
        .then((data) => {
          setAvailableSlots(data.slots || []);
          setLoading(false);
        })
        .catch((err) => {
          console.error(err);
          setAvailableSlots(['10:00', '11:30', '14:00', '15:30']); // Fallback
          setLoading(false);
        });
    }
  }, [selectedDate]);

  const handleBook = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedDate || !selectedSlot) return;

    setBookingStatus('submitting');

    const timeStr = `${format(selectedDate, 'yyyy-MM-dd')}T${selectedSlot}:00Z`;

    try {
      const response = await fetch('http://localhost:8080/api/book', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: formData.name,
          email: formData.email,
          time: timeStr,
          platform: formData.platform,
        }),
      });

      if (response.ok) {
        setBookingStatus('success');
      } else {
        setBookingStatus('error');
      }
    } catch (err) {
      setBookingStatus('error');
    }
  };

  if (bookingStatus === 'success') {
    return (
      <motion.div
        initial={{ opacity: 0, scale: 0.98 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
        className="text-center py-12 px-6 border border-gray-800 bg-[#080808] rounded-sm"
      >
        <div className="w-16 h-16 border border-gray-700 text-gray-400 rounded-full flex items-center justify-center mx-auto mb-8 shadow-inner shadow-gray-900">
          <Calendar size={28} strokeWidth={1.5} />
        </div>
        <h2 className="text-2xl font-serif text-white mb-4 italic tracking-wide">Appointment Confirmed.</h2>
        <p className="text-gray-500 font-light leading-relaxed mb-8">
          The details have been dispatched to <span className="text-white">{formData.email}</span>.<br/>
          Scheduled for <span className="text-white">{selectedDate && format(selectedDate, 'MMM d, yyyy')}</span> at <span className="text-white">{selectedSlot}</span> via <span className="text-white">{formData.platform}</span>.
        </p>
        <button
          onClick={() => {
            setBookingStatus('idle');
            setSelectedDate(null);
            setSelectedSlot(null);
          }}
          className="uppercase tracking-[0.2em] text-xs py-4 px-8 border border-white text-white hover:bg-white hover:text-black transition-colors rounded-sm font-medium w-full sm:w-auto"
        >
          Book Another
        </button>
      </motion.div>
    );
  }

  return (
    <div className="w-full">
      <AnimatePresence mode="wait">
        {!selectedDate ? (
          <motion.div
            key="date-selection"
            initial={{ opacity: 0, y: 5 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -5 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
            className="grid grid-cols-2 sm:grid-cols-4 gap-[2px] bg-gray-900 border border-gray-900 p-[2px] rounded-sm"
          >
            {nextDates.map((date) => (
              <button
                key={date.toISOString()}
                onClick={() => setSelectedDate(date)}
                className="flex flex-col items-center justify-center py-6 px-2 bg-[#080808] hover:bg-[#111] transition-colors group relative overflow-hidden rounded-sm"
              >
                <span className="text-[10px] text-gray-500 group-hover:text-gray-400 uppercase tracking-[0.2em] font-medium mb-2">
                  {format(date, 'EEE')}
                </span>
                <span className="text-3xl font-serif text-white group-hover:text-gray-200 transition-colors">
                  {format(date, 'd')}
                </span>
                <span className="text-[10px] text-gray-600 group-hover:text-gray-500 uppercase tracking-[0.2em] font-medium mt-2">
                  {format(date, 'MMM')}
                </span>
              </button>
            ))}
          </motion.div>
        ) : !selectedSlot ? (
          <motion.div
            key="time-selection"
            initial={{ opacity: 0, x: 10 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -10 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          >
            <div className="flex items-center justify-between mb-8 pb-4 border-b border-gray-900">
              <h3 className="text-xs font-medium text-gray-400 uppercase tracking-[0.2em] flex items-center gap-3">
                <Clock size={14} className="text-gray-500" />
                Slots • {format(selectedDate, 'MMMM d')}
              </h3>
              <button
                onClick={() => setSelectedDate(null)}
                className="text-[10px] uppercase tracking-[0.2em] font-medium text-gray-500 hover:text-white transition-colors"
              >
                Return
              </button>
            </div>

            {loading ? (
              <div className="flex justify-center py-12">
                <div className="animate-pulse h-px w-24 bg-gray-600"></div>
              </div>
            ) : availableSlots.length > 0 ? (
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                {availableSlots.map((slot) => (
                  <button
                    key={slot}
                    onClick={() => setSelectedSlot(slot)}
                    className="py-4 px-2 border border-gray-800 bg-[#080808] hover:bg-white hover:text-black hover:border-white text-gray-300 transition-all text-sm font-medium tracking-widest rounded-sm"
                  >
                    {slot}
                  </button>
                ))}
              </div>
            ) : (
              <div className="text-center py-12 border border-gray-900 bg-[#0a0a0a] rounded-sm text-gray-500 text-sm tracking-wide font-light">
                No availability. Choose another date.
              </div>
            )}
          </motion.div>
        ) : (
          <motion.div
            key="form"
            initial={{ opacity: 0, scale: 0.98 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          >
            <div className="flex items-center justify-between mb-8 pb-4 border-b border-gray-900">
              <div className="flex flex-col">
                <span className="text-[10px] uppercase tracking-[0.2em] font-medium text-gray-600 mb-2">Finalization</span>
                <span className="text-sm text-gray-200 font-light flex items-center gap-3">
                  <Calendar size={14} className="text-gray-500"/> {format(selectedDate, 'MMM d, yyyy')} • {selectedSlot}
                </span>
              </div>
              <button
                onClick={() => setSelectedSlot(null)}
                className="text-[10px] uppercase tracking-[0.2em] font-medium text-gray-500 hover:text-white transition-colors"
              >
                Change Time
              </button>
            </div>

            <form onSubmit={handleBook} className="space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                <div>
                  <label className="block text-[10px] uppercase tracking-[0.2em] font-medium text-gray-500 mb-2">Full Name</label>
                  <input
                    type="text"
                    required
                    className="w-full px-0 pb-3 pt-1 bg-transparent border-b border-gray-800 text-white focus:ring-0 focus:border-white outline-none transition-colors placeholder:text-gray-700 text-sm font-light rounded-none"
                    placeholder="Enter your name"
                    value={formData.name}
                    onChange={(e) => setFormData({...formData, name: e.target.value})}
                  />
                </div>

                <div>
                  <label className="block text-[10px] uppercase tracking-[0.2em] font-medium text-gray-500 mb-2">Email Address</label>
                  <input
                    type="email"
                    required
                    className="w-full px-0 pb-3 pt-1 bg-transparent border-b border-gray-800 text-white focus:ring-0 focus:border-white outline-none transition-colors placeholder:text-gray-700 text-sm font-light rounded-none"
                    placeholder="Enter your email"
                    value={formData.email}
                    onChange={(e) => setFormData({...formData, email: e.target.value})}
                  />
                </div>
              </div>

              <div className="pt-4">
                <label className="block text-[10px] uppercase tracking-[0.2em] font-medium text-gray-500 mb-4">Select Venue</label>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    onClick={() => setFormData({...formData, platform: 'Google Meet'})}
                    className={`flex flex-col items-center justify-center gap-3 py-6 px-4 border transition-all rounded-sm ${
                      formData.platform === 'Google Meet'
                        ? 'border-white bg-[#111] text-white'
                        : 'border-gray-800 bg-[#080808] text-gray-500 hover:border-gray-600'
                    }`}
                  >
                    <Video size={20} strokeWidth={1.5} />
                    <span className="text-[10px] uppercase tracking-[0.2em] font-medium">Meet</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setFormData({...formData, platform: 'Telegram'})}
                    className={`flex flex-col items-center justify-center gap-3 py-6 px-4 border transition-all rounded-sm ${
                      formData.platform === 'Telegram'
                        ? 'border-white bg-[#111] text-white'
                        : 'border-gray-800 bg-[#080808] text-gray-500 hover:border-gray-600'
                    }`}
                  >
                    <MessageCircle size={20} strokeWidth={1.5} />
                    <span className="text-[10px] uppercase tracking-[0.2em] font-medium">Telegram</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setFormData({...formData, platform: 'Instagram'})}
                    className={`flex flex-col items-center justify-center gap-3 py-6 px-4 border transition-all rounded-sm ${
                      formData.platform === 'Instagram'
                        ? 'border-white bg-[#111] text-white'
                        : 'border-gray-800 bg-[#080808] text-gray-500 hover:border-gray-600'
                    }`}
                  >
                    <Instagram size={20} strokeWidth={1.5} />
                    <span className="text-[10px] uppercase tracking-[0.2em] font-medium">Instagram</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setFormData({...formData, platform: 'Phone'})}
                    className={`flex flex-col items-center justify-center gap-3 py-6 px-4 border transition-all rounded-sm ${
                      formData.platform === 'Phone'
                        ? 'border-white bg-[#111] text-white'
                        : 'border-gray-800 bg-[#080808] text-gray-500 hover:border-gray-600'
                    }`}
                  >
                    <Phone size={20} strokeWidth={1.5} />
                    <span className="text-[10px] uppercase tracking-[0.2em] font-medium">Phone</span>
                  </button>
                </div>
              </div>

              <div className="pt-8">
                <button
                  type="submit"
                  disabled={bookingStatus === 'submitting'}
                  className="w-full py-5 px-6 bg-white hover:bg-gray-200 text-black text-sm tracking-[0.2em] uppercase font-bold flex items-center justify-center gap-3 transition-colors disabled:opacity-50 rounded-sm"
                >
                  {bookingStatus === 'submitting' ? 'Confirming...' : 'Lock the Appointment'}
                  <ArrowRight size={18} strokeWidth={2} />
                </button>
              </div>
            </form>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
